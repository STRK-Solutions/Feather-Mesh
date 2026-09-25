"""Fixtures test the probe itself; they are not deployed Cloudflare evidence."""
import base64
import json
import os
from pathlib import Path
import tempfile
import socket
import threading
import time
import unittest
from unittest.mock import patch

import probe

WS = "u-44444444-4444-4444-8444-444444444444.613202690.xyz"
OTHER = "u-55555555-5555-4555-8555-555555555555.613202690.xyz"


def config():
    return {"schema_version": 1, "issuer": "https://fixture-team.cloudflareaccess.com", "workspace": WS,
            "other_workspace": OTHER, "audiences": {probe.PORTAL: "a" * 64, probe.ADMIN: "b" * 64, WS: "c" * 64, OTHER: "d" * 64}, "tokens": {}}


def token(host=WS, subject="fixture-owner", expired=False, lifetime=3600):
    now = int(time.time()) - (7200 if expired else 0)
    payload = {"iss": config()["issuer"], "aud": [config()["audiences"][host]], "type": "app", "sub": subject, "iat": now, "exp": now + lifetime}
    encode = lambda v: base64.urlsafe_b64encode(json.dumps(v).encode()).decode().rstrip("=")
    return encode({"alg": "RS256"}) + "." + encode(payload) + ".ZmFrZXNpZ25hdHVyZQ"


def passing(case, headers, mode):
    status = case.expected[0]
    values = {}
    if status == 101:
        values = {"sec-websocket-accept": probe.ACCEPT, "upgrade": "websocket", "connection": "Upgrade"}
    if case.mutation == "redirect":
        values["location"] = "https://" + case.host + case.path
    return status, values


class ProbeTests(unittest.TestCase):
    def test_real_socket_http_and_upgrade_transport(self):
        for response, expected in ((b"HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n", 403),
                                   (("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + probe.ACCEPT + "\r\n\r\n").encode(), 101)):
            client, server = socket.socketpair()
            received = []
            class Connected:
                def connect(self, address):
                    self.address = address
                def __getattr__(self, name):
                    return getattr(client, name)
            def respond():
                with server:
                    data = b""
                    while b"\r\n\r\n" not in data:
                        data += server.recv(4096)
                    received.append(data)
                    server.sendall(response)
                    self.assertEqual(server.recv(1), b"")  # Never send terminal input.
            worker = threading.Thread(target=respond)
            worker.start()
            conn = Connected()
            case = probe.Case("actual_socket", WS, path="/ws", ws=True, origin="https://" + WS)
            with patch("probe.socket.socket", return_value=conn):
                status, values = probe.request(case, probe.headers(case, {}, "origin"), "origin")
            worker.join(timeout=3)
            self.assertFalse(worker.is_alive())
            self.assertEqual(status, expected)
            self.assertEqual(conn.address, probe.SOCKET)
            self.assertTrue(received[0].startswith(b"GET /ws HTTP/1.1\r\n"))
            if status == 101:
                self.assertEqual(values["sec-websocket-accept"], probe.ACCEPT)

    def test_real_wire_repeatable_telemetry_does_not_allow_authoritative_duplicates(self):
        case = probe.Case("headers", WS)

        def exchange(response):
            client, server = socket.socketpair()
            class Connected:
                def connect(self, address):
                    pass
                def __getattr__(self, name):
                    return getattr(client, name)
            def respond():
                with server:
                    data = b""
                    while b"\r\n\r\n" not in data:
                        data += server.recv(4096)
                    server.sendall(response)
            worker = threading.Thread(target=respond)
            worker.start()
            try:
                with patch("probe.socket.socket", return_value=Connected()):
                    return probe.request(case, probe.headers(case, {}, "origin"), "origin")
            finally:
                worker.join(timeout=3)
                self.assertFalse(worker.is_alive())

        wire = (b"HTTP/1.1 302 Found\r\n"
                b"Server-Timing: cfL4;desc=abc\r\nserver-timing: cfEdge;dur=1\r\n"
                b"Set-Cookie: a=1\r\nSet-Cookie: b=2\r\n"
                b"Location: https://fixture-team.cloudflareaccess.com/cdn-cgi/access/login\r\n\r\n")
        status, parsed = exchange(wire)
        self.assertEqual(status, 302)
        self.assertEqual(parsed, {"location": "https://fixture-team.cloudflareaccess.com/cdn-cgi/access/login"})
        duplicates = (
            b"HTTP/1.1 302 Found\r\nServer-Timing: cfL4\r\n"
            b"Location: https://evil.invalid\r\n"
            b"Location: https://fixture-team.cloudflareaccess.com/cdn-cgi/access/login\r\n\r\n",
            b"HTTP/1.1 101 Switching Protocols\r\nServer-Timing: cfL4\r\n"
            b"Sec-WebSocket-Accept: bad\r\nSec-WebSocket-Accept: worse\r\n\r\n",
        )
        for wire in duplicates:
            with self.assertRaises(ValueError):
                exchange(wire)
        with self.assertRaises(ValueError):
            exchange(b"HTTP/1.1 403 Forbidden\r\nServer-Timing\r\n\r\n")

    def test_all_cases_and_receipt_are_sanitized(self):
        tokens = {c.token: token() for c in probe.cases(config(), "public") if c.token}
        result = probe.run(config(), tokens, "public", passing)
        self.assertTrue(result["passed"])
        self.assertGreaterEqual(len(result["cases"]), 25)
        encoded = json.dumps(result)
        self.assertNotIn("fixture-owner", encoded)
        self.assertNotIn(token(), encoded)
        self.assertNotIn("headers", encoded)

    def test_failures_cannot_pass_as_denial_or_unavailable(self):
        case = probe.Case("no_login", WS)
        for mode in ("origin", "public"):
            for status in (101, 200, 404, 429, 500, 502, 503):
                self.assertFalse(probe.verdict(case, status, {}, mode, config()["issuer"]))
        self.assertFalse(probe.run(config(), {}, "origin", passing)["passed"])
        def failed(*args):
            raise OSError("SECRET must never appear")
        result = probe.run(config(), {}, "public", failed)
        self.assertNotIn("SECRET", json.dumps(result))
        self.assertFalse(result["passed"])

    def test_access_redirect_is_exact_issuer_and_login_route(self):
        case = probe.Case("no_login", WS)
        good = {"location": config()["issuer"] + "/cdn-cgi/access/login/" + WS}
        self.assertTrue(probe.verdict(case, 302, good, "public", config()["issuer"]))
        self.assertFalse(probe.verdict(case, 302, good, "origin", config()["issuer"]))
        for url in ("http://fixture-team.cloudflareaccess.com/cdn-cgi/access/login", "https://evil.invalid/cdn-cgi/access/login", config()["issuer"] + "/wrong", config()["issuer"] + "@evil.invalid/cdn-cgi/access/login"):
            self.assertFalse(probe.verdict(case, 302, {"location": url}, "public", config()["issuer"]))

    def test_redirect_preserves_host_path_query(self):
        case = probe.Case("redirect", WS, path="/path?q=1", expected=(301, 308), mutation="redirect")
        self.assertTrue(probe.verdict(case, 301, {"location": "https://" + WS + "/path?q=1"}, "public", ""))
        for location in ("https://" + WS + "/path", "https://" + probe.PORTAL + "/path?q=1", "http://" + WS + "/path?q=1"):
            self.assertFalse(probe.verdict(case, 301, {"location": location}, "public", ""))

    def test_websocket_requires_correct_handshake(self):
        case = probe.Case("ws", WS, expected=(101,), ws=True)
        self.assertTrue(probe.verdict(case, *passing(case, {}, "public"), "public", ""))
        self.assertFalse(probe.verdict(case, 101, {}, "public", ""))
        self.assertFalse(probe.verdict(case, 200, {}, "public", ""))

    def test_tokens_not_sent_on_http_redirect_and_auth_layers_differ(self):
        t = {"workspace": token()}
        case = probe.Case("owner", WS, "workspace")
        self.assertEqual(probe.headers(case, t, "origin")["Cf-Access-Jwt-Assertion"], t["workspace"])
        self.assertEqual(probe.headers(case, t, "public")["Cookie"], "CF_Authorization=" + t["workspace"])
        for case in probe.cases(config(), "public"):
            if case.mutation == "redirect":
                self.assertNotIn("Cookie", probe.headers(case, t, "public"))
            if case.method == "POST":
                self.assertEqual(probe.headers(case, {"portal": token(probe.PORTAL)}, "public")["Content-Length"], "0")
        tampered = probe.headers(probe.Case("bad", WS, "workspace", mutation="signature"), t, "origin")
        self.assertNotEqual(tampered["Cf-Access-Jwt-Assertion"], t["workspace"])

    def test_private_config_and_actual_expiry_and_identity(self):
        with tempfile.TemporaryDirectory() as temp:
            p = Path(temp) / "config.json"
            p.write_text(json.dumps(config()))
            p.chmod(0o600)
            self.assertEqual(probe.read_config(p), config())
            p.chmod(0o644)
            with self.assertRaises(ValueError):
                probe.read_config(p)
            p.chmod(0o600)
            link = Path(temp) / "symlink"
            link.symlink_to(p)
            with self.assertRaises(OSError):
                probe.read_config(link)
            t = Path(temp) / "jwt"
            t.write_text(token())
            t.chmod(0o600)
            c = config()
            c["tokens"] = {"workspace": str(t), "cross_user": str(t)}
            with self.assertRaises(ValueError):
                probe.load_tokens(c)
            c["tokens"] = {"expired_workspace": str(t)}
            with self.assertRaises(ValueError):
                probe.load_tokens(c)
            t.write_text(token(expired=True))
            self.assertIn("expired_workspace", probe.load_tokens(c))
            c["tokens"] = {"admin": str(t)}
            with self.assertRaises(ValueError):
                probe.load_tokens(c)

    def test_config_closed_hosts_unique_audiences_and_scope(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "config.json"
            for change in (lambda c: c.update(extra=True), lambda c: c["audiences"].update({"evil.invalid": "e" * 64}),
                           lambda c: c["audiences"].update({probe.ADMIN: "a" * 64}), lambda c: c.update(issuer="https://fixture-team.cloudflareaccess.com/"),
                           lambda c: c.update(other_workspace=WS)):
                c = config()
                change(c)
                path.write_text(json.dumps(c))
                path.chmod(0o600)
                with self.assertRaises(ValueError):
                    probe.read_config(path)

    def test_actual_ingress_snapshot_has_no_bypass(self):
        aud = config()["audiences"]
        rows = [{"hostname": host, "service": "unix:" + probe.SOCKET,
                 "originRequest": {"access": {"required": True, "teamName": "fixture-team", "audTag": [value]}}} for host, value in aud.items()]
        rows.append({"service": "http_status:404"})
        self.assertTrue(probe.ingress_check({"result": {"config": {"ingress": rows}}}, aud, True, "fixture-team"))
        for mutate in (lambda r: r.pop(), lambda r: r[0].update(service="http://localhost:8080"),
                       lambda r: r[0]["originRequest"]["access"].update(required=False),
                       lambda r: r[0]["originRequest"]["access"].update(audTag=["other"]),
                       lambda r: r[0].update(hostname="*.613202690.xyz"), lambda r: r[0].update(path="/public")):
            altered = json.loads(json.dumps(rows))
            mutate(altered)
            self.assertFalse(probe.ingress_check({"ingress": altered}, aud, True, "fixture-team"))
        for row in rows:
            row["service"] = "http_status:404"
        self.assertTrue(probe.ingress_check({"ingress": rows}, aud, False, "fixture-team"))

    def test_direct_origin_ambiguous_network_errors_fail(self):
        with patch("probe.socket.create_connection", side_effect=OSError("no local route")):
            self.assertFalse(probe.origin_ports("192.0.2.1")["passed"])
        with patch("probe.socket.create_connection", side_effect=ConnectionRefusedError()):
            result = probe.origin_ports("192.0.2.1")
            self.assertTrue(result["passed"])
            self.assertEqual(len(result["ports"]), 6)


if __name__ == "__main__":
    unittest.main()
