"""Fast negative tests for storage ownership admission; actual mounts use VM proof."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from types import SimpleNamespace

spec=importlib.util.spec_from_file_location('storage',Path(__file__).resolve().parents[2]/'scripts/w1_storage.py')
storage=importlib.util.module_from_spec(spec);spec.loader.exec_module(storage)

class StorageAdmissionTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name)
        self.scope=patch.object(storage,'ROOT',self.root);self.scope.start();self.addCleanup(self.scope.stop)

    def test_unrecorded_file_never_formatted(self):
        file=self.root/'runtime.ext4';file.write_bytes(b'owned by someone else')
        with patch.object(storage,'safe_root'),patch.object(storage.subprocess,'run') as mutation:
            with self.assertRaisesRegex(ValueError,'unrecorded'):
                storage.provision('ignored',4096,2048)
            mutation.assert_not_called()
        self.assertEqual(file.read_bytes(),b'owned by someone else')

    def test_symlink_backing_never_formatted(self):
        target=self.root/'other';target.write_text('preserve')
        (self.root/'runtime.ext4').symlink_to(target)
        with patch.object(storage,'safe_root'),patch.object(storage.subprocess,'run') as mutation:
            with self.assertRaisesRegex(ValueError,'unrecorded'):
                storage.provision('ignored',4096,2048)
            mutation.assert_not_called()
        self.assertEqual(target.read_text(),'preserve')

    def test_incomplete_inventory_fails_verification(self):
        with patch.object(storage,'safe_root'):
            with self.assertRaisesRegex(ValueError,'incomplete'):
                storage.verify('ignored')

    def test_pool_never_shrinks_or_accepts_unbounded_count(self):
        with patch.object(storage, 'safe_root'), patch.object(storage, 'load_inventory', return_value={'volumes': {'slot-12': {}}}):
            with self.assertRaisesRegex(ValueError, 'shrink'):
                storage.provision('ignored', 20480, 2048, 2)
            with self.assertRaisesRegex(ValueError, 'finite pool'):
                storage.provision('ignored', 20480, 2048, 13)

    def test_pool_checks_all_collisions_before_first_allocation(self):
        (self.root/'slot-12.ext4').write_text('unowned')
        with patch.object(storage, 'safe_root'), patch.object(storage, 'format_new') as mutation:
            with self.assertRaisesRegex(ValueError, 'unrecorded'):
                storage.provision('ignored', 20480, 2048, 12)
            mutation.assert_not_called()
        self.assertFalse((self.root/'runtime.ext4').exists())

    def test_wrong_parent_refused_before_mutation(self):
        with patch.object(storage.os,'geteuid',return_value=0),patch.object(storage,'run',return_value='wrong'),patch.object(Path,'is_symlink',return_value=False):
            with self.assertRaisesRegex(ValueError,'UUID mismatch'):
                storage.safe_root('expected')

    def test_parent_uuid_is_resolved_at_service_root(self):
        with patch.object(storage.os,'geteuid',return_value=0), patch.object(storage,'run',return_value='wrong') as read_mount, patch.object(Path,'is_symlink',return_value=False):
            with self.assertRaisesRegex(ValueError,'UUID mismatch'):
                storage.safe_root('expected')
            read_mount.assert_called_once_with('findmnt','-n','-o','UUID','--target',str(self.root))

    def test_pending_inode_initialization_refused(self):
        for output in ('', 'Group 0: [ITABLE_ZEROED]\nGroup 1: [INODE_UNINIT]'):
            with self.subTest(output=output), patch.object(storage.subprocess,'run',return_value=SimpleNamespace(stdout=output,returncode=0)):
                with self.assertRaisesRegex(ValueError,'initialization incomplete'):
                    storage.initialized(self.root/'recorded.ext4')

    def test_initialized_groups_admitted(self):
        with patch.object(storage.subprocess,'run',return_value=SimpleNamespace(stdout='Group 0: [ITABLE_ZEROED]\nGroup 1: [INODE_UNINIT, ITABLE_ZEROED]',returncode=0)):
            storage.initialized(self.root/'recorded.ext4')

    def test_failed_scan_never_records_initialization(self):
        with patch.object(storage.subprocess,'run',return_value=SimpleNamespace(stdout='Group 0: [ITABLE_ZEROED]',stderr='bitmap checksum mismatch',returncode=156)):
            with self.assertRaisesRegex(ValueError,'bitmap checksum mismatch'):
                storage.initialized(self.root/'recorded.ext4')

if __name__=='__main__':unittest.main()
