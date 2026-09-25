import hashlib
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from types import SimpleNamespace

spec = importlib.util.spec_from_file_location('pipeline_tools', Path(__file__).resolve().parents[2] / 'scripts/install_pipeline_tools.py')
tools = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tools)


class PipelineToolsTests(unittest.TestCase):
    def test_exact_bounded_sources_and_symlink_rejection(self):
        with tempfile.TemporaryDirectory() as root:
            p = Path(root) / 'source'
            p.write_bytes(b'reviewed')
            digest = hashlib.sha256(b'reviewed').hexdigest()
            self.assertEqual(tools.checked_source(p, digest), b'reviewed')
            with self.assertRaisesRegex(ValueError, 'differs'):
                tools.checked_source(p, '0' * 64)
            q = Path(root) / 'link'
            q.symlink_to(p)
            with self.assertRaisesRegex(ValueError, 'regular'):
                tools.checked_source(q, digest)

    def test_foreign_target_rejected_before_environment_creation(self):
        with tempfile.TemporaryDirectory() as root:
            parent = Path(root)
            source = parent / 'source'
            source.write_bytes(b'locked')
            digest = hashlib.sha256(b'locked').hexdigest()
            link = parent / 'current'
            link.write_text('preserve')
            with patch.object(tools, 'ROOT', parent / 'releases'), patch.object(tools, 'LINKS', {link: 'venv'}), patch.object(tools, 'private_root'), patch.object(tools.os, 'geteuid', return_value=0), patch.object(tools.sys, 'platform', 'linux'), patch.object(tools.sys, 'version_info', (3, 12)), patch.object(tools.platform, 'machine', return_value='x86_64'), patch.object(tools.subprocess, 'run') as mutation:
                with self.assertRaisesRegex(ValueError, 'non-link'):
                    tools.install(source, digest, source, digest)
                mutation.assert_not_called()
            self.assertEqual(link.read_text(), 'preserve')

    def test_unrecorded_symlink_rejected_before_download(self):
        with tempfile.TemporaryDirectory() as root:
            parent = Path(root)
            source = parent / 'source'
            source.write_bytes(b'locked')
            digest = hashlib.sha256(b'locked').hexdigest()
            link = parent / 'current'
            link.symlink_to(parent / 'unrelated/venv')
            with patch.object(tools, 'ROOT', parent / 'releases'), patch.object(tools, 'LINKS', {link: 'venv'}), patch.object(tools, 'private_root'), patch.object(tools.os, 'geteuid', return_value=0), patch.object(tools.sys, 'platform', 'linux'), patch.object(tools.sys, 'version_info', (3, 12)), patch.object(tools.platform, 'machine', return_value='x86_64'), patch.object(tools.subprocess, 'run') as mutation:
                with self.assertRaisesRegex(ValueError, 'unowned'):
                    tools.install(source, digest, source, digest)
                mutation.assert_not_called()

    def test_explicit_ensurepip_reconciliation_preserves_partial_bytes(self):
        with tempfile.TemporaryDirectory() as root:
            releases = Path(root)
            destination = releases / ('a' * 64)
            (destination / 'venv').mkdir(parents=True)
            (destination / 'venv/pyvenv.cfg').write_text('partial venv')
            (destination / 'requirements.lock').write_bytes(b'locked')
            (destination / 'convert.py').write_bytes(b'converter')
            sources = {'requirements.lock': b'locked', 'convert.py': b'converter'}
            real_stat = Path.stat

            def root_owned(path, *args, **kwargs):
                info = real_stat(path, *args, **kwargs)
                return SimpleNamespace(st_mode=info.st_mode, st_uid=0)

            with patch.object(tools, 'ROOT', releases), patch.object(Path, 'stat', root_owned):
                (destination / 'unreviewed').write_text('keep')
                with self.assertRaisesRegex(ValueError, 'unexpected'):
                    tools.archive_incomplete_venv(destination, sources)
                self.assertTrue(destination.exists())
                (destination / 'unreviewed').unlink()
                archive = tools.archive_incomplete_venv(destination, sources)
            self.assertFalse(destination.exists())
            self.assertEqual(archive.name, 'a' * 64 + '.incomplete-ensurepip')
            self.assertEqual((archive / 'venv/pyvenv.cfg').read_text(), 'partial venv')
            self.assertEqual((archive / 'convert.py').read_bytes(), b'converter')


if __name__ == '__main__':
    unittest.main()
