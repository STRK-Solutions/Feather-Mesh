"""Local route-boundary tests; mounted Linux acceptance is separate."""
import importlib.util
import json
from pathlib import Path
import tempfile
import tomllib
import unittest

ROOT = Path(__file__).resolve().parents[4]
spec = importlib.util.spec_from_file_location('attach_releases', ROOT / 'infra/demo/images/terminal/attach_releases.py')
attach = importlib.util.module_from_spec(spec)
spec.loader.exec_module(attach)


class ReleaseAttachmentTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        root = Path(self.temporary.name)
        self.project = root / 'client with spaces'
        (self.project / '.feam').mkdir(parents=True)
        self.path = self.project / '.feam/project.toml'
        self.practice = ('schema_version = 1\nnamespace = "consumer"\nowner_teams = ["consumer"]\n'
                         '\n# Participant practice stays intact.\n[[peers]]\nalias = "nearby-climate"\n'
                         'namespace = "climate"\npath = "peers/nearby-climate"\n')
        self.path.write_text(self.practice)
        self.datasets = root / 'datasets'
        serving = self.datasets / 'demo-climate/serving'
        serving.mkdir(parents=True)
        (serving / 'manifest.json').write_text(json.dumps({'schema_version': 1, 'namespace': 'demo-climate', 'revision': 2}))
        self.config = root / 'releases.json'
        self.release = {'bundle': 'demo-climate', 'namespace': 'demo-climate', 'digest': 'a' * 64,
                        'serving_root': '/datasets/demo-climate/serving'}
        self.config.write_text(json.dumps([self.release]))

    def run_attach(self):
        attach.attach(self.project, self.config, self.datasets)

    def test_attach_replace_revoke_preserve_practice_and_idempotency(self):
        self.run_attach()
        first = self.path.read_bytes()
        self.run_attach()
        self.assertEqual(first, self.path.read_bytes())
        config = tomllib.loads(first.decode())
        self.assertEqual(config['peers'][0]['path'], 'peers/nearby-climate')
        self.assertEqual(config['peers'][1]['path'], '/datasets/demo-climate/serving')
        self.assertIn(self.practice.rstrip(), first.decode())
        self.release['digest'] = 'b' * 64
        self.config.write_text(json.dumps([self.release]))
        self.run_attach()
        self.assertNotIn('a' * 64, self.path.read_text())
        self.config.write_text('[]')
        self.run_attach()
        self.assertEqual(len(tomllib.loads(self.path.read_text())['peers']), 1)

    def test_invalid_identity_path_manifest_and_conflict_fail_without_change(self):
        for key, value in [('serving_root', '/etc'), ('serving_root', '/datasets/demo-climate/../other/serving'),
                           ('namespace', 'climate'), ('digest', 'bad'), ('bundle', '../demo-climate')]:
            invalid = dict(self.release, **{key: value})
            self.config.write_text(json.dumps([invalid]))
            with self.assertRaises(ValueError):
                self.run_attach()
            self.assertEqual(self.path.read_text(), self.practice)
        self.config.write_text(json.dumps([self.release, self.release]))
        with self.assertRaises(ValueError):
            self.run_attach()
        self.config.write_text(json.dumps([self.release]))
        self.path.write_text(self.practice.replace('namespace = "climate"', 'namespace = "demo-climate"'))
        with self.assertRaises(ValueError):
            self.run_attach()
        self.path.write_text(self.practice)
        manifest = self.datasets / 'demo-climate/serving/manifest.json'
        manifest.write_text('{"schema_version":1,"namespace":"wrong","revision":1}')
        with self.assertRaises(ValueError):
            self.run_attach()

    def test_symlink_and_ambiguous_managed_section_denied(self):
        target = self.path.with_name('other.toml')
        self.path.rename(target)
        self.path.symlink_to(target)
        with self.assertRaises(ValueError):
            self.run_attach()
        self.path.unlink()
        self.path.write_text(self.practice + attach.BEGIN)
        with self.assertRaises(ValueError):
            self.run_attach()
        self.path.write_text(self.practice)
        serving = self.datasets / 'demo-climate/serving'
        target = serving.with_name('other')
        serving.rename(target)
        serving.symlink_to(target, target_is_directory=True)
        with self.assertRaises(ValueError):
            self.run_attach()


if __name__ == '__main__':
    unittest.main()
