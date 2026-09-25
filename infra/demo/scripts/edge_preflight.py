#!/usr/bin/env python3
"""Fixed-scope root installer/start guard for the FEAM Cloudflare connector."""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import pwd
import re
import stat
import subprocess
import tempfile
import time
import urllib.request
from urllib.parse import urlsplit

VERSION = '2026.9.3'
SHA256 = '77e26d8d900e0b8469f416239d14b5f296525fdf79fee6f511ef55609e3fbac2'
SIZE = 40122749
URL = 'https://github.com/cloudflare/cloudflared/releases/download/' + VERSION + '/cloudflared-linux-amd64'
BINARY = Path('/opt/feam/edge/cloudflared-' + VERSION)
CONFIG = Path('/etc/feam/edge/config.json')
TOKEN = Path('/etc/feam/edge/tunnel-token')
MANIFEST = Path('/etc/feam/services/runtime.json')
UID = 2109


def require(condition, message):
    if not condition:
        raise ValueError(message)


def owned(path, private=False, directory=False):
    info = path.lstat()
    require(info.st_uid == 0 and not info.st_mode & (0o077 if private else 0o022), 'root ownership or mode mismatch')
    require(stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode), 'unexpected path type')
    return info


def read_private(path, limit):
    info = owned(path, private=True)
    require(info.st_size <= limit, 'private input too large')
    raw = path.read_bytes()
    require(len(raw) <= limit, 'private input too large')
    return raw


def validate_identity(entry):
    require(entry.pw_name == 'feam-edge' and entry.pw_uid == UID and entry.pw_gid == UID and
            entry.pw_dir == '/nonexistent' and entry.pw_shell == '/usr/sbin/nologin', 'dedicated frontend identity mismatch')


def validate_config(value):
    require(isinstance(value, dict) and set(value) == {'protocol', 'active', 'account_id', 'tunnel_id', 'readiness_sha256'}, 'unknown edge configuration fields')
    require(value['protocol'] == 'feam.edge.v1' and value['active'] is True, 'connector activation is disabled')
    require(re.fullmatch(r'[0-9a-f]{32}', value['account_id']) and
            re.fullmatch(r'[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}', value['tunnel_id']) and
            re.fullmatch(r'[0-9a-f]{64}', value['readiness_sha256']), 'reviewed account, tunnel and readiness receipt required')


def validate_token(raw, value):
    require(len(raw) <= 16384, 'tunnel token too large')
    token = raw.strip()
    decoded = json.loads(base64.b64decode(token + b'=' * (-len(token) % 4), validate=True))
    require(isinstance(decoded, dict) and set(decoded) == {'a', 't', 's'} and
            decoded['a'] == value['account_id'] and decoded['t'] == value['tunnel_id'] and
            isinstance(decoded['s'], str) and 16 <= len(decoded['s']) <= 1024, 'token does not match the recorded tunnel')
    require(16 <= len(base64.b64decode(decoded['s'], validate=True)) <= 128, 'invalid tunnel secret encoding')


def verify_binary():
    info = owned(BINARY)
    require(info.st_size == SIZE and stat.S_IMODE(info.st_mode) == 0o555, 'pinned executable size or mode mismatch')
    with BINARY.open('rb') as f:
        require(hashlib.file_digest(f, 'sha256').hexdigest() == SHA256, 'pinned executable digest mismatch')


class OfficialRedirects(urllib.request.HTTPRedirectHandler):
    max_redirections = 3
    max_repeats = 1

    def redirect_request(self, req, fp, code, msg, headers, newurl):
        parsed = urlsplit(newurl)
        require(parsed.scheme == 'https' and parsed.hostname in ('github.com', 'release-assets.githubusercontent.com', 'objects.githubusercontent.com') and
                parsed.port in (None, 443) and not parsed.username and not parsed.password, 'unexpected artifact redirect')
        return super().redirect_request(req, fp, code, msg, headers, newurl)


def fetch():
    for parent in (Path('/opt'), Path('/opt/feam'), BINARY.parent):
        owned(parent, directory=True)
    if BINARY.exists() or BINARY.is_symlink():
        verify_binary()  # Never overwrite an unfamiliar existing artifact.
        return
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), OfficialRedirects())
    deadline = time.monotonic() + 180
    temp_path = None
    try:
        with opener.open(urllib.request.Request(URL, headers={'User-Agent': 'feam-edge-installer/1'}), timeout=15) as response:
            length = response.headers.get('Content-Length')
            require(length is None or int(length) == SIZE, 'unexpected artifact byte count')
            with tempfile.NamedTemporaryFile(prefix='.cloudflared-', dir=BINARY.parent, delete=False) as output:
                temp_path = Path(output.name)
                digest, count = hashlib.sha256(), 0
                while True:
                    require(time.monotonic() < deadline, 'artifact deadline exceeded')
                    # read1 returns after one underlying read; a slowly trickled
                    # response cannot extend a full buffered read indefinitely.
                    chunk = response.read1(min(65536, SIZE - count + 1))
                    require(time.monotonic() < deadline, 'artifact deadline exceeded')
                    if not chunk:
                        break
                    count += len(chunk)
                    require(count <= SIZE, 'artifact exceeds pinned size')
                    output.write(chunk)
                    digest.update(chunk)
                require(count == SIZE and digest.hexdigest() == SHA256, 'artifact size or digest mismatch')
                output.flush()
                os.fchmod(output.fileno(), 0o555)
                os.fsync(output.fileno())
        os.link(temp_path, BINARY)  # Atomic no-replace publication in the same directory.
        parent_fd = os.open(BINARY.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(parent_fd)
        finally:
            os.close(parent_fd)
    finally:
        if temp_path:
            temp_path.unlink(missing_ok=True)
    verify_binary()


def ready():
    for parent in (Path('/etc/feam'), CONFIG.parent, BINARY.parent):
        owned(parent, directory=True)
    validate_identity(pwd.getpwnam('feam-edge'))
    config = json.loads(read_private(CONFIG, 65536))
    validate_config(config)
    manifest = json.loads(read_private(MANIFEST, 1048576))
    require(manifest.get('protocol') == 'feam.services.v1' and manifest.get('frontend_uid') == UID, 'gateway frontend ACL identity does not match connector')
    validate_token(read_private(TOKEN, 16384), config)
    verify_binary()
    browser = Path('/run/feam/services/gateway/browser.sock').lstat()
    require(stat.S_ISSOCK(browser.st_mode) and browser.st_uid == 2102, 'gateway browser socket is not ready')
    # Check the kernel's actual named-UID ACL, not just the manifest intention.
    result = subprocess.run(['/usr/bin/setpriv', '--reuid=' + str(UID), '--regid=' + str(UID), '--clear-groups',
                             '/usr/bin/test', '-w', '/run/feam/services/gateway/browser.sock'],
                            stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                            timeout=5, check=False)
    require(result.returncode == 0, 'connector cannot access its gateway socket')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('fetch', 'verify-binary', 'ready'))
    args = parser.parse_args()
    try:
        require(os.geteuid() == 0, 'root operator required')
        {'fetch': fetch, 'verify-binary': verify_binary, 'ready': ready}[args.action]()
    except (OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError):
        raise SystemExit('FEAM edge preflight failed; inspect the private installation, identity and approval record') from None
    print('FEAM edge ' + args.action + ' passed')


if __name__ == '__main__':
    main()
