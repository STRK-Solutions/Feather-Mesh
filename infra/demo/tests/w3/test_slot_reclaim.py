"""Unprivileged finite cleanup fixtures; no actual host mount or Docker proof."""
import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[2] / 'scripts/slot_reclaim.py'
spec = importlib.util.spec_from_file_location('slot_reclaim', SCRIPT)
reclaim = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reclaim)


class ReclaimTests(unittest.TestCase):
    def test_removes_private_entries_without_following_symlinks(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            slot = root / 'slot'
            slot.mkdir()
            outside = root / 'outside'
            outside.write_text('preserved')
            (slot / 'link').symlink_to(outside)
            (slot / 'directory').mkdir()
            (slot / 'directory/data').write_text('disposable')
            reclaim.clear_contents(slot)
            self.assertEqual(list(slot.iterdir()), [])
            self.assertEqual(outside.read_text(), 'preserved')

    def test_symlink_root_and_unknown_lost_found_fail_before_deletion(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            target = root / 'target'
            target.mkdir()
            (target / 'lost+found').write_text('retain')
            (target / 'private').write_text('retain')
            with self.assertRaises(ValueError):
                reclaim.clear_contents(target)
            self.assertTrue((target / 'private').exists())
            (root / 'link').symlink_to(target, target_is_directory=True)
            with self.assertRaises(OSError):
                reclaim.clear_contents(root / 'link')

    def test_any_runtime_mount_reference_blocks_even_stopped_container(self):
        slot = Path('/home/feam-service-data/slot-01')
        for source in [str(slot), str(slot / 'private'), str(slot.parent)]:
            with patch.object(reclaim, 'runtime_json', side_effect=[
                    [{'Id': 'a' * 64}], {'Mounts': [{'Source': source}]}]):
                with self.assertRaises(ValueError):
                    reclaim.require_no_container_mount(slot)
        with patch.object(reclaim, 'runtime_json', return_value=[]):
            reclaim.require_no_container_mount(slot)


if __name__ == '__main__':
    unittest.main()
