"""Offline connector installer/guard tests. No host mutation or edge connection."""
import base64
import copy
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import types
import unittest
from unittest import mock
import urllib.request

ROOT = Path(__file__).resolve().parents[4]
spec = importlib.util.spec_from_file_location('edge_preflight', ROOT / 'infra/demo/scripts/edge_preflight.py')
edge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(edge)


def config():
    return {'protocol': 'feam.edge.v1', 'active': True, 'account_id': 'a' * 32,
            'tunnel_id': '11111111-1111-4111-8111-111111111111', 'readiness_sha256': 'b' * 64}


def token(**changes):
    value = {'a': config()['account_id'], 't': config()['tunnel_id'], 's': base64.b64encode(b's' * 32).decode()}
    value.update(changes)
    return base64.b64encode(json.dumps(value).encode())


class ConnectorTests(unittest.TestCase):
    def test_fixed_binary_pin_and_disabled_install_defaults(self):
        import yaml
        play = yaml.safe_load((ROOT / 'infra/demo/ansible/edge.yml').read_text())[0]
        for name in ('start', 'enable', 'activate', 'stop'):
            self.assertIs(play['vars']['feam_edge_' + name], False)
        self.assertEqual(edge.VERSION, '2026.9.3')
        self.assertEqual(edge.SIZE, 40122749)
        self.assertEqual(edge.SHA256, '77e26d8d900e0b8469f416239d14b5f296525fdf79fee6f511ef55609e3fbac2')
        self.assertEqual(edge.UID, 2109)
        self.assertEqual(str(edge.MANIFEST), '/etc/feam/services/runtime.json')

    def test_activation_and_token_are_exact_and_closed(self):
        edge.validate_config(config())
        edge.validate_token(token(), config())
        for key, value in (('active', False), ('active', 1), ('protocol', 'other'), ('readiness_sha256', ''),
                           ('account_id', 'outside'), ('tunnel_id', '../other'), ('command', 'shell')):
            changed = config()
            changed[key] = value
            with self.assertRaises(ValueError):
                edge.validate_config(changed)
        for bad in (token(a='c' * 32), token(t='22222222-2222-4222-8222-222222222222'),
                    token(e='https://outside.invalid'), token(s='not-a-base64-secret!'), b'bad', b'a' * 16385):
            with self.assertRaises(ValueError):
                edge.validate_token(bad, config())

    def test_frontend_identity_has_no_login_or_operator_home(self):
        value = types.SimpleNamespace(pw_name='feam-edge', pw_uid=2109, pw_gid=2109,
                                      pw_dir='/nonexistent', pw_shell='/usr/sbin/nologin')
        edge.validate_identity(value)
        for key, changed in (('pw_uid', 2108), ('pw_gid', 2108), ('pw_dir', '/home/operator'), ('pw_shell', '/bin/bash')):
            bad = copy.copy(value)
            setattr(bad, key, changed)
            with self.assertRaises(ValueError):
                edge.validate_identity(bad)

    def test_download_redirects_never_send_to_arbitrary_destinations(self):
        handler = edge.OfficialRedirects()
        request = urllib.request.Request(edge.URL)
        for url in ('http://github.com/file', 'https://169.254.169.254/token', 'https://github.com.evil.invalid/file',
                    'https://user:secret@github.com/file', 'https://github.com:8443/file'):
            with self.assertRaises(ValueError):
                handler.redirect_request(request, None, 302, '', {}, url)
        result = handler.redirect_request(request, None, 302, '', {}, 'https://release-assets.githubusercontent.com/pinned-object')
        self.assertEqual(result.host, 'release-assets.githubusercontent.com')
        self.assertEqual(handler.max_redirections, 3)

    def test_bounded_fetch_publishes_only_exact_hash_and_never_replaces(self):
        class Response(io.BytesIO):
            headers = {}
        with tempfile.TemporaryDirectory() as temp:
            target = Path(temp) / 'cloudflared'
            data = b'fixture native bytes'
            fake = mock.Mock()
            with mock.patch.object(edge, 'BINARY', target), mock.patch.object(edge, 'SIZE', len(data)), \
                 mock.patch.object(edge, 'SHA256', hashlib.sha256(data).hexdigest()), \
                 mock.patch.object(edge, 'owned'), mock.patch.object(edge, 'verify_binary') as verify, \
                 mock.patch.object(edge.urllib.request, 'build_opener', return_value=fake):
                fake.open.return_value = Response(data)
                edge.fetch()
                self.assertEqual(target.read_bytes(), data)
                self.assertEqual(target.stat().st_mode & 0o777, 0o555)
                self.assertEqual(list(Path(temp).glob('.cloudflared-*')), [])
                edge.fetch()
                self.assertEqual(fake.open.call_count, 1)
                verify.assert_called()
                target.unlink()
                for body in (data + b'extra', b'wrong bytes', data[:-1]):
                    fake.open.return_value = Response(body)
                    with self.assertRaises(ValueError):
                        edge.fetch()
                    self.assertFalse(target.exists())
                    self.assertEqual(list(Path(temp).glob('.cloudflared-*')), [])

    def test_unit_has_no_key_in_argv_and_no_privileged_connector(self):
        unit = (ROOT / 'infra/demo/ansible/roles/edge/templates/feam-edge.service.j2').read_text()
        for value in ('User=feam-edge', 'Group=feam-edge', 'ProtectSystem=strict', 'ProtectHome=yes',
                      'CapabilityBoundingSet=\n', 'NoNewPrivileges=yes', '--no-autoupdate',
                      'LoadCredential=tunnel-token:/etc/feam/edge/tunnel-token',
                      '--token-file ${CREDENTIALS_DIRECTORY}/tunnel-token', '--metrics 127.0.0.1:20241',
                      'ExecStartPre=+/usr/local/sbin/feam-edge-preflight ready', 'MemoryMax=256M'):
            self.assertIn(value, unit)
        self.assertNotIn('--token ', unit)
        self.assertNotIn('EnvironmentFile=', unit)
        self.assertNotIn('User=root', unit)
        self.assertNotIn('--loglevel debug', unit)
        self.assertNotIn('ReadWritePaths=', unit)


if __name__ == '__main__':
    unittest.main()
