"""Copy-up alias checks; real namespace acceptance remains an Ubuntu check."""
import importlib.util
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[4]
spec = importlib.util.spec_from_file_location('rootless_mount', ROOT / 'infra/demo/scripts/rootless_run_mount.py')
helper = importlib.util.module_from_spec(spec)
spec.loader.exec_module(helper)


class CopyupTests(unittest.TestCase):
    def check(self, link='.ro730973553/feam', uid=2101, output=b'112:1\n', code=0, mounts=None):
        target = mock.Mock()
        target.lstat.return_value = SimpleNamespace(st_uid=uid, st_gid=2101)
        with mock.patch.object(helper.os, 'readlink', return_value=link), \
             mock.patch.object(helper, 'target_mounts', return_value=mounts or []), \
             mock.patch.object(helper.subprocess, 'run', return_value=SimpleNamespace(returncode=code, stdout=output)) as run:
            helper.verify_copyup_link(123, target, SimpleNamespace(st_dev=112, st_ino=1), '0:112')
            args = run.call_args.args[0]
            self.assertIn('--user=/proc/123/ns/user', args)
            self.assertIn('--mount=/proc/123/ns/mnt', args)
            self.assertEqual(args[-4:], ['/usr/bin/stat', '-Lc', '%d:%i', '/run/feam'])

    def test_exact_copyup_alias_verified_inside_daemon_namespace(self):
        self.check()

    def test_reject_foreign_alias_owner_mount_or_namespace_result(self):
        for kwargs in ({'link': '/run/feam'}, {'link': '../feam'}, {'link': '.ro1/other'},
                       {'uid': 0}, {'output': b'113:1\n'}, {'output': b'112:2\n'},
                       {'code': 1}, {'mounts': [('unexpected', 'mount')]}):
            with self.subTest(kwargs=kwargs), self.assertRaises(ValueError):
                self.check(**kwargs)


if __name__ == '__main__':
    unittest.main()
