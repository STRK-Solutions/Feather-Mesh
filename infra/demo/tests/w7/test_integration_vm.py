import importlib.util
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('integration_vm', Path(__file__).resolve().parents[2] / 'scripts/integration_vm.py')
vm = importlib.util.module_from_spec(spec)
spec.loader.exec_module(vm)


class IntegrationVMTests(unittest.TestCase):
    def test_existing_disk_never_reinitialized(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'vm.qcow2').write_bytes(b'preserve')
            with patch.object(vm, 'ROOT', root), patch.object(vm.base, 'run') as mutation:
                with self.assertRaisesRegex(ValueError, 'never reinitialize'):
                    vm.prepare(root / 'probe')
                mutation.assert_not_called()
            self.assertEqual((root / 'vm.qcow2').read_bytes(), b'preserve')

    def test_host_floor_is_in_addition_to_guest_maximum(self):
        with patch.object(vm.os, 'statvfs', return_value=SimpleNamespace(f_bavail=450, f_blocks=1000, f_frsize=vm.GIB)):
            with self.assertRaisesRegex(ValueError, 'storage floor'):
                vm.disk_headroom()
        with patch.object(vm.os, 'statvfs', return_value=SimpleNamespace(f_bavail=600, f_blocks=1000, f_frsize=vm.GIB)):
            vm.disk_headroom()

    def test_foreign_pid_is_not_controlled(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            pid = root / 'qemu.pid'
            pid.write_text('1234')
            pid.chmod(0o600)
            proc = root / 'proc/1234'
            proc.mkdir(parents=True)
            (proc / 'cmdline').write_bytes(b'other-process\x00')
            def path(value):
                return root / 'proc' if value == '/proc' else Path(value)
            with patch.object(vm, 'ROOT', root), patch.object(vm, 'Path', side_effect=path):
                with self.assertRaisesRegex(ValueError, 'different process'):
                    vm.running_pid()


if __name__ == '__main__':
    unittest.main()
