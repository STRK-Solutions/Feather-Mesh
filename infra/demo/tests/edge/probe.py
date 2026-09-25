#!/usr/bin/env python3
"""Bounded, read-only W8 admission probes. Never print tokens, emails or bodies."""
from __future__ import annotations

import argparse
import base64
from dataclasses import dataclass
import hashlib
import ipaddress
import json
import os
from pathlib import Path
import re
import socket
import ssl
import stat
import time
from urllib.parse import urlsplit

PORTAL = "feam.613202690.xyz"
ADMIN = "admin.613202690.xyz"
WORKSPACE = re.compile(r"u-[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\.613202690\.xyz\Z")
SOCKET = "/run/feam/services/gateway/browser.sock"
KEY = "dGhlIHNhbXBsZSBub25jZQ=="
ACCEPT = base64.b64encode(hashlib.sha1((KEY + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11").encode()).digest()).decode()
DENY = (401, 403)


def closed(value, keys):
    if not isinstance(value, dict) or set(value) - set(keys):
        raise ValueError("unknown or invalid configuration fields")


def private_read(path, limit):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        st = os.fstat(fd)
        if not stat.S_ISREG(st.st_mode) or st.st_uid != os.getuid() or st.st_mode & 0o077 or st.st_size > limit:
            raise ValueError("input must be an owner-only bounded regular file")
        with os.fdopen(fd, "rb", closefd=False) as f:
            result = f.read(limit + 1)
            if len(result) > limit:
                raise ValueError("input exceeded the byte limit")
            return result
    finally:
        os.close(fd)


def read_config(path):
    value = json.loads(private_read(path, 65536))
    closed(value, ("schema_version", "issuer", "audiences", "workspace", "other_workspace", "tokens", "origin_ip"))
    if value.get("schema_version") != 1 or not re.fullmatch(r"https://[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.cloudflareaccess\.com", value.get("issuer", "")):
        raise ValueError("invalid schema version or issuer")
    audiences = value.get("audiences", {})
    if not isinstance(audiences, dict) or not 3 <= len(audiences) <= 12:
        raise ValueError("three to twelve exact host audiences required")
    for host, aud in audiences.items():
        if host not in (PORTAL, ADMIN) and not WORKSPACE.fullmatch(host):
            raise ValueError("hostname outside the exact demo scope")
        if not isinstance(aud, str) or not re.fullmatch(r"[0-9a-f]{64}", aud):
            raise ValueError("record each exact Cloudflare application audience")
    if len(set(audiences.values())) != len(audiences) or not {PORTAL, ADMIN, value.get("workspace")} <= set(audiences):
        raise ValueError("distinct portal, admin and workspace audiences required")
    if not WORKSPACE.fullmatch(value.get("workspace", "")):
        raise ValueError("record a workspace host")
    other = value.get("other_workspace")
    if other and (other == value["workspace"] or other not in audiences or not WORKSPACE.fullmatch(other)):
        raise ValueError("second workspace must be distinct and recorded")
    closed(value.get("tokens", {}), ("portal", "admin", "workspace", "other_workspace", "cross_user", "expired_workspace", "disabled_workspace"))
    if value.get("origin_ip"):
        ipaddress.ip_address(value["origin_ip"])
    return value


def load_tokens(config):
    tokens, subjects = {}, {}
    for name, path in config.get("tokens", {}).items():
        token = private_read(path, 16384).decode().strip()
        if not re.fullmatch(r"[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+", token):
            raise ValueError("input must contain only one signed Access assertion")
        payload = token.split(".")[1]
        claims = json.loads(base64.urlsafe_b64decode(payload + "=" * (-len(payload) % 4)))
        host = {"portal": PORTAL, "admin": ADMIN, "other_workspace": config.get("other_workspace")}.get(name, config["workspace"])
        if claims.get("iss") != config["issuer"] or claims.get("aud") != [config["audiences"][host]] or claims.get("type") != "app" or not claims.get("sub"):
            raise ValueError("token metadata does not match its exact host; signature is checked by the service")
        iat, exp = claims.get("iat"), claims.get("exp")
        if type(iat) is not int or type(exp) is not int or not 0 < exp - iat <= (3600 if name == "admin" else 28800):
            raise ValueError("invalid application token lifetime")
        if name == "expired_workspace":
            if exp > time.time():
                raise ValueError("expired probe needs an actually expired signed token")
        elif exp <= time.time() or iat > time.time() + 5:
            raise ValueError("positive probe token is expired or future dated")
        tokens[name], subjects[name] = token, claims["sub"]
    if "cross_user" in subjects and subjects.get("workspace") == subjects["cross_user"]:
        raise ValueError("cross-user probe requires a different subject with the same workspace audience")
    if "cross_user" in subjects and subjects.get("other_workspace") != subjects["cross_user"]:
        raise ValueError("cross-user token must match the second owner's positive workspace token")
    return tokens


@dataclass(frozen=True)
class Case:
    name: str
    host: str
    token: str | None = None
    path: str = "/"
    method: str = "GET"
    origin: str | None = None
    expected: tuple[int, ...] = DENY
    ws: bool = False
    mutation: str = ""


def cases(config, mode):
    ws = config["workspace"]
    out = [Case("portal_without_login", PORTAL), Case("forged_email_header", PORTAL, mutation="email"),
           Case("admin_without_login", ADMIN), Case("workspace_without_login", ws),
           Case("portal_user", PORTAL, "portal", expected=(200,)), Case("administrator", ADMIN, "admin", expected=(200,)),
           Case("workspace_owner", ws, "workspace", expected=(200,)),
           Case("portal_audience_cannot_admin", ADMIN, "portal"), Case("admin_audience_cannot_workspace", ws, "admin"),
           Case("workspace_audience_cannot_portal", PORTAL, "workspace"),
           Case("wrong_signature", ws, "workspace", mutation="signature"),
           Case("unsigned_assertion", ws, "workspace", mutation="unsigned"),
           Case("expired_assertion", ws, "expired_workspace"), Case("disabled_account", ws, "disabled_workspace"),
           Case("cross_user_same_audience", ws, "cross_user"), Case("admin_no_terminal_same_audience", ws),
           Case("terminal_unknown_route", ws, "workspace", path="/__feam_probe_missing__", expected=(404,)),
           Case("ws_owner", ws, "workspace", path="/ws", origin="https://" + ws, expected=(101,), ws=True),
           Case("ws_missing_origin", ws, "workspace", path="/ws", ws=True),
           Case("ws_sibling_origin", ws, "workspace", path="/ws", origin="https://" + PORTAL, ws=True),
           Case("ws_no_login", ws, path="/ws", origin="https://" + ws, ws=True),
           Case("csrf_sibling_origin", PORTAL, "portal", path="/workspace/action", method="POST", origin="https://" + ws)]
    if config.get("other_workspace"):
        out.append(Case("other_workspace_audience", config["other_workspace"], "workspace"))
        out.append(Case("second_workspace_owner", config["other_workspace"], "other_workspace", expected=(200,)))
    if mode == "origin":
        out.append(Case("unknown_host_denied", "unmatched.613202690.xyz", "workspace"))
    if mode == "public":
        for host in config["audiences"]:
            out.append(Case("https_redirect_" + host, host, path="/__feam_redirect_probe__?a=1&b=two", expected=(301, 308), mutation="redirect"))
    return out


def headers(case, tokens, mode):
    out = {"Host": case.host, "Connection": "close", "User-Agent": "feam-w8-probe/1"}
    if case.token:
        token = tokens[case.token]
        if case.mutation == "signature":
            first, payload, signature = token.split(".")
            token = first + "." + payload + "." + ("A" if signature[0] != "A" else "B") + signature[1:]
        elif case.mutation == "unsigned":
            token = "eyJhbGciOiJub25lIn0." + token.split(".")[1] + "."
        out["Cf-Access-Jwt-Assertion" if mode == "origin" else "Cookie"] = token if mode == "origin" else "CF_Authorization=" + token
    if case.mutation == "email":
        out["Cf-Access-Authenticated-User-Email"] = "forged-probe@example.invalid"
    if case.origin:
        out["Origin"] = case.origin
    if case.method == "POST":
        out["Content-Length"] = "0"  # No action, workspace, CSRF, or confirmation: cannot mutate.
    if case.ws:
        out.update({"Connection": "Upgrade", "Upgrade": "websocket", "Sec-WebSocket-Key": KEY,
                    "Sec-WebSocket-Version": "13", "Sec-WebSocket-Protocol": "tty"})
    return out


def request(case, values, mode):
    # TLS hostname and CA verification are mandatory; no redirects are followed.
    deadline = time.monotonic() + 8
    raw = socket.socket(socket.AF_UNIX) if mode == "origin" else socket.socket(socket.AF_INET)
    raw.settimeout(5)
    try:
        if mode == "origin":
            raw.connect(SOCKET)
        else:
            raw.close()
            raw = socket.create_connection((case.host, 80 if case.mutation == "redirect" else 443), timeout=5)
            if case.mutation != "redirect":
                raw = ssl.create_default_context().wrap_socket(raw, server_hostname=case.host)
        data = f"{case.method} {case.path} HTTP/1.1\r\n" + "".join(f"{k}: {v}\r\n" for k, v in values.items()) + "\r\n"
        raw.sendall(data.encode("ascii"))
        buf = b""
        while b"\r\n\r\n" not in buf:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError("response deadline")
            raw.settimeout(min(5, remaining))
            block = raw.recv(1)  # Do not capture terminal payload or response bodies.
            if not block or len(buf) >= 32768:
                raise ValueError("missing or oversized response headers")
            buf += block
        lines = buf.decode("iso-8859-1").split("\r\n")
        match = re.fullmatch(r"HTTP/1\.[01] ([0-9]{3})(?: .*)?", lines[0])
        if not match:
            raise ValueError("invalid HTTP status")
        response_headers = {}
        for line in lines[1:]:
            if not line:
                continue
            key, sep, value = line.partition(":")
            key = key.lower()
            if not sep:
                raise ValueError("ambiguous response headers")
            # These repeatable, non-authoritative fields are never retained.
            if key in ("set-cookie", "server-timing"):
                continue
            if key in response_headers:
                raise ValueError("ambiguous response headers")
            response_headers[key] = value.strip()
        return int(match[1]), response_headers
    finally:
        raw.close()  # Immediate close after WS handshake; never send terminal input.


def verdict(case, status, values, mode, issuer):
    if case.mutation == "redirect":
        return status in case.expected and values.get("location") == "https://" + case.host + case.path
    if status in case.expected:
        if status == 101:
            return (values.get("sec-websocket-accept") == ACCEPT and values.get("upgrade", "").lower() == "websocket"
                    and "upgrade" in values.get("connection", "").lower().split(","))
        return True
    if mode == "public" and case.expected == DENY and status in (302, 303, 307):
        target = urlsplit(values.get("location", ""))
        return target.scheme == "https" and target.netloc == urlsplit(issuer).netloc and target.path.startswith("/cdn-cgi/access/login")
    return False


def run(config, tokens, mode, transport=request):
    result = []
    deadline = time.monotonic() + 180
    for case in cases(config, mode):
        row = {"case": case.name, "passed": False}
        if case.token and case.token not in tokens:
            row["error"] = "required_private_token_missing"
        elif time.monotonic() >= deadline:
            row["error"] = "suite_deadline"
        else:
            try:
                status, response = transport(case, headers(case, tokens, mode), mode)
                row.update(status=status, passed=verdict(case, status, response, mode, config["issuer"]))
            except (OSError, ValueError, UnicodeError):
                row["error"] = "transport_or_protocol_failed"
        result.append(row)
    return {"schema_version": 1, "mode": mode, "time_unix": int(time.time()),
            "passed": all(r["passed"] for r in result), "cases": result}


def ingress_check(snapshot, audiences, selected, team_name):
    """Validate the actual API configuration, not merely the Terraform source."""
    config = snapshot.get("result", snapshot)
    config = config.get("config", config)
    ingress = config.get("ingress", [])
    if not isinstance(ingress, list) or len(ingress) != len(audiences) + 1:
        return False
    if ingress[-1] != {"service": "http_status:404"}:
        return False
    seen = set()
    for row in ingress[:-1]:
        host = row.get("hostname")
        if host not in audiences or host in seen or row.get("path"):
            return False
        seen.add(host)
        if row.get("service") != ("unix:" + SOCKET if selected else "http_status:404"):
            return False
        origin = row.get("originRequest", row.get("origin_request", {}))
        access = origin.get("access", {})
        aud = access.get("audTag", access.get("aud_tag"))
        team = access.get("teamName", access.get("team_name"))
        if access.get("required") is not True or aud != [audiences[host]] or team != team_name:
            return False
        if origin.get("httpHostHeader") or origin.get("http_host_header"):
            return False
    return seen == set(audiences)


def origin_ports(ip):
    # Fixed small set, one explicitly named operator origin; SSH is not scanned.
    target = str(ipaddress.ip_address(ip))
    rows = []
    for port in (80, 443, 7681, 8080, 2375, 2376):
        try:
            with socket.create_connection((target, port), timeout=3):
                reachable = True
        except (ConnectionRefusedError, TimeoutError):
            reachable = False
        except OSError:
            rows.append({"port": port, "passed": False, "error": "network_unverifiable"})
            continue
        rows.append({"port": port, "passed": not reachable})
    return {"schema_version": 1, "mode": "origin-ports", "passed": all(r["passed"] for r in rows), "ports": rows,
            "scope": "only these ports from this probe vantage point; pair with host socket/firewall audit"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mode", choices=("public", "origin", "ingress", "origin-ports"), required=True)
    parser.add_argument("--config", required=True)
    parser.add_argument("--snapshot", help="private GET tunnel/configurations JSON for ingress mode")
    parser.add_argument("--selected", action="store_true", help="snapshot is the selected tunnel")
    args = parser.parse_args()
    try:
        config = read_config(args.config)
        if args.mode == "ingress":
            snapshot = json.loads(private_read(args.snapshot, 262144))
            team_name = urlsplit(config["issuer"]).hostname.removesuffix(".cloudflareaccess.com")
            result = {"schema_version": 1, "mode": "ingress", "passed": ingress_check(snapshot, config["audiences"], args.selected, team_name)}
        elif args.mode == "origin-ports":
            result = origin_ports(config["origin_ip"])
        else:
            result = run(config, load_tokens(config), args.mode)
    except (KeyError, TypeError, OSError, ValueError, UnicodeError):
        result = {"schema_version": 1, "mode": args.mode, "passed": False, "error": "invalid_private_configuration"}
    print(json.dumps(result, indent=2, sort_keys=True))
    raise SystemExit(0 if result["passed"] else 1)


if __name__ == "__main__":
    main()
