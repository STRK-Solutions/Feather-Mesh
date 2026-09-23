"""Destructive demo reset stays confined to its explicitly marked root."""
from pathlib import Path
import os
import subprocess
import tempfile
import unittest

SCRIPT=Path(__file__).resolve().parent/'tui_agent_stage1_demo.sh'

class DemoResetTests(unittest.TestCase):
    def test_unmarked_wrong_marker_and_symlink_roots_preserve_bytes(self):
        with tempfile.TemporaryDirectory() as tmp:
            base=Path(tmp); root=base/'demo';root.mkdir();keep=root/'provider';keep.mkdir();sentinel=keep/'keep';sentinel.write_bytes(b'original')
            for marker in [None,'wrong-marker']:
                if marker: (root/'.feam-stage1-demo-marker').write_text(marker)
                result=subprocess.run([str(SCRIPT),str(root),'--reset'],capture_output=True)
                self.assertNotEqual(result.returncode,0);self.assertEqual(sentinel.read_bytes(),b'original')
            (root/'.feam-stage1-demo-marker').write_text('feam-stage1-demo-v1\n')
            link=base/'alias';link.symlink_to(root,target_is_directory=True)
            result=subprocess.run([str(SCRIPT),str(link),'--reset'],capture_output=True)
            self.assertNotEqual(result.returncode,0);self.assertEqual(sentinel.read_bytes(),b'original')
            marker=root/'.feam-stage1-demo-marker';marker.unlink();marker.symlink_to(sentinel)
            result=subprocess.run([str(SCRIPT),str(root),'--reset'],capture_output=True)
            self.assertNotEqual(result.returncode,0);self.assertEqual(sentinel.read_bytes(),b'original')

    def test_marked_reset_unlinks_child_symlink_without_following_it(self):
        with tempfile.TemporaryDirectory() as tmp:
            base=Path(tmp);outside=base/'outside';outside.mkdir();sentinel=outside/'keep';sentinel.write_bytes(b'original')
            root=base/'demo';root.mkdir();(root/'.feam-stage1-demo-marker').write_text('feam-stage1-demo-v1\n')
            (root/'provider').symlink_to(outside,target_is_directory=True)
            # Stop after reset and before setup: the sentinel must survive.
            result=subprocess.run([str(SCRIPT),str(root),'--reset'],capture_output=True,env=dict(os.environ,FEAM_EXECUTABLE=str(base/'absent')))
            self.assertNotEqual(result.returncode,0);self.assertEqual(sentinel.read_bytes(),b'original');self.assertFalse((root/'provider').is_symlink())

if __name__=='__main__':unittest.main()
