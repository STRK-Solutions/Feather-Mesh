"""Local contract tests; runtime image/mount acceptance is a separate gate."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest

ROOT = Path(__file__).resolve().parents[4]
IMAGE = ROOT / 'infra/demo/images/terminal'
spec = importlib.util.spec_from_file_location('hosted_check', IMAGE / 'hosted_check.py')
hosted = importlib.util.module_from_spec(spec)
spec.loader.exec_module(hosted)


class HostedImageTests(unittest.TestCase):
    def test_fixed_profile_and_missing_hosted_inputs_fail_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            config = Path(directory)
            (config / 'feam').mkdir()
            for name in ('ca.pem', 'cert.pem', 'key.pem', 'capability'):
                (config / name).write_text('a' * 43)
                (config / name).chmod(0o600)
            profile = (IMAGE / 'agent.toml.example').read_text()
            (config / 'feam/agent.toml').write_text(profile)
            socket = SimpleNamespace(lstat=lambda: SimpleNamespace(st_mode=stat.S_IFSOCK))
            hosted.validate(config, socket)
            (config / 'capability').chmod(0o640)
            hosted.validate(config, socket)  # named-ACL class; host proves exact reader
            (config / 'capability').write_text('pending-generation-activation')
            hosted.validate(config, socket)  # staged placeholder grants no request authority
            (config / 'capability').write_text('a' * 43)
            for original, replacement in [
                ('https://127.0.0.1:8443/v1', 'https://unapproved.invalid/v1'),
                ('deepinfra/fp8', 'unapproved'),
                ('allow_provider_fallbacks = false', 'allow_provider_fallbacks = true'),
            ]:
                (config / 'feam/agent.toml').write_text(profile.replace(original, replacement))
                with self.assertRaises(ValueError):
                    hosted.validate(config, socket)
            (config / 'feam/agent.toml').write_text(profile)
            (config / 'capability').chmod(0o644)
            with self.assertRaises(ValueError):
                hosted.validate(config, socket)
            (config / 'capability').unlink()
            with self.assertRaises(OSError):
                hosted.validate(config, socket)

    def test_manual_w1_launcher_remains_explicit(self):
        with tempfile.TemporaryDirectory() as directory:
            directory = Path(directory)
            executable = directory / 'feam'
            executable.write_text('#!/bin/sh\nprintf "%s\\n" "$@" > "$TEST_ARGS"\n')
            executable.chmod(0o700)
            env = dict(os.environ, PATH=str(directory) + os.pathsep + os.environ['PATH'],
                       TEST_ARGS=str(directory / 'arguments'), FEAM_DEMO_MODE='manual')
            result = subprocess.run(['bash', str(IMAGE / 'launcher.sh')], env=env,
                                    stdin=subprocess.DEVNULL, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual((directory / 'arguments').read_text().splitlines(),
                             ['tui', '--project', '/workspace/demo/client with spaces', '--agent', 'off'])
            self.assertIn('Manual mode', result.stdout)

    def test_context_rejects_unreviewed_or_wrong_architecture_artifacts(self):
        with tempfile.TemporaryDirectory() as directory:
            directory = Path(directory)
            # Header-only synthetic fixtures test rejection, not executable quality.
            elf = b'\x7fELF\x02\x01' + b'\0' * 12 + b'\x3e\x00' + b'fixture'
            paths = {name: directory / name for name in ('feam', 'ttyd', 'model-adapter')}
            for path in paths.values():
                path.write_bytes(elf)
            manifest = {name: hashlib.sha256(elf).hexdigest() for name in paths}
            record = directory / 'artifacts.json'
            record.write_text(json.dumps(manifest))
            command = [sys.executable, str(ROOT / 'infra/demo/scripts/build_terminal_context.py')]
            for name, path in paths.items():
                command += ['--' + name, str(path)]
            command += ['--artifact-manifest', str(record), '--output', str(directory / 'output')]
            paths['feam'].write_bytes(elf + b'changed')
            result = subprocess.run(command, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('does not match', result.stderr)
            paths['feam'].write_bytes(b'Mach-O example')
            result = subprocess.run(command, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('native Linux amd64', result.stderr)
            self.assertFalse((directory / 'output').exists())


if __name__ == '__main__':
    unittest.main()
