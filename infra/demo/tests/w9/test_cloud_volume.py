"""No root, real device, mount, format or provider mutation in these fixtures."""
import importlib.util
import json
from pathlib import Path
import subprocess
import stat
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[2] / 'scripts/cloud_volume.py'
spec = importlib.util.spec_from_file_location('cloud_volume', SCRIPT)
volume = importlib.util.module_from_spec(spec)
spec.loader.exec_module(volume)


def manifest():
    deployment = '11111111-1111-4111-8111-111111111111'
    return {'protocol': 'feam.cloud-volume.v1', 'deployment_id': deployment,
            'volume_id': '22222222-2222-4222-8222-222222222222', 'droplet_id': '123',
            'approved_quote_sha256': 'a' * 64, 'region': 'nyc3',
            'volume_name': 'feam-' + deployment + '-service', 'bytes': 320 << 30,
            'filesystem_uuid': deployment, 'mount': '/home/feam-service-data'}


class CloudVolumeTests(unittest.TestCase):
    def test_wrong_size_mounted_partitioned_or_readonly_devices_are_refused(self):
        base = {'name': '/dev/fixture', 'type': 'disk', 'size': 320 << 30,
                'ro': False, 'pkname': None, 'mountpoints': [None]}
        for change in [{'size': 319 << 30}, {'mountpoints': ['/existing']},
                       {'children': [{}]}, {'ro': True}, {'type': 'part'}]:
            observed = json.dumps({'blockdevices': [{**base, **change}]})
            with self.subTest(change=change), \
                    patch.object(Path, 'is_symlink', return_value=True), \
                    patch.object(Path, 'resolve', return_value=Path('/dev/fixture')), \
                    patch.object(Path, 'stat', return_value=SimpleNamespace(st_mode=stat.S_IFBLK, st_rdev=42)), \
                    patch.object(volume, 'run', return_value=subprocess.CompletedProcess([], 0, observed, '')):
                with self.assertRaises(ValueError):
                    volume.device_inventory(manifest())

    def test_scope_rejects_expansion_and_unknown_identity(self):
        self.assertEqual(volume.validate_manifest(manifest()), manifest())
        for key, value in [('bytes', 321 << 30), ('mount', '/home'), ('region', 'tor1'),
                           ('volume_name', 'unknown'), ('filesystem_uuid', 'unknown'),
                           ('approved_quote_sha256', 'unapproved'), ('droplet_id', 'shell;')]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                volume.validate_manifest({**manifest(), key: value})

    def test_existing_signature_or_ambiguous_probe_is_never_blank(self):
        device = Path('/dev/fixture')
        for signature in [{'type': 'ext4'}, {'type': 'gpt'}]:
            with patch.object(volume, 'run', return_value=subprocess.CompletedProcess([], 0,
                              json.dumps({'signatures': [signature]}), '')) as run:
                with self.assertRaises(ValueError):
                    volume.require_blank(device)
                self.assertEqual(run.call_count, 1)
        for code in [0, 1, 8]:
            results = [subprocess.CompletedProcess([], 0, '{"signatures":[]}', ''),
                       subprocess.CompletedProcess([], code, '', '')]
            with patch.object(volume, 'run', side_effect=results), self.assertRaises(ValueError):
                volume.require_blank(device)

    def test_existing_intent_and_wrong_resource_block_all_format_commands(self):
        with tempfile.TemporaryDirectory() as temporary:
            state = Path(temporary) / 'ownership.json'
            state.write_text('{}')
            with patch.object(volume, 'STATE', state), patch.object(volume, 'run') as run:
                with self.assertRaises(ValueError):
                    volume.initialize(manifest(), 'a' * 64, manifest()['volume_id'])
                with self.assertRaises(ValueError):
                    volume.initialize(manifest(), 'a' * 64, 'unowned')
                run.assert_not_called()

    def test_format_intent_survives_interruption_and_prevents_replay(self):
        with tempfile.TemporaryDirectory() as temporary:
            state = Path(temporary) / 'ownership.json'
            device = Path('/dev/fixture')
            with patch.object(volume, 'STATE', state), \
                    patch.object(volume, 'device_inventory', return_value=(device, 42)), \
                    patch.object(volume, 'require_blank'), \
                    patch.object(volume, 'run', side_effect=RuntimeError('interrupted')) as run:
                with self.assertRaises(RuntimeError):
                    volume.initialize(manifest(), 'a' * 64, manifest()['volume_id'])
                self.assertTrue(state.exists())
                with self.assertRaises(ValueError):
                    volume.initialize(manifest(), 'a' * 64, manifest()['volume_id'])
                self.assertEqual(run.call_count, 1)

    def test_capacity_preserves_floor_and_additional_metadata_headroom(self):
        # Conservative models, not measured cloud capacity. Actual admission
        # uses statvfs after initialization, and W1 validates the physical pool.
        with self.assertRaises(ValueError):
            volume.capacity(315 << 30, 299 << 30, volume.POOL)
        result = volume.capacity(318 << 30, 318 << 30, volume.POOL)
        self.assertGreater(result['surplus_bytes'], 0)
        with self.assertRaises(ValueError):
            volume.capacity(320 << 30, volume.POOL + 48 * volume.GIB, volume.POOL)

    def test_measured_native_format_model_covers_the_conservative_floor(self):
        evidence = Path(__file__).resolve().parents[4] / 'docs/evaluations/web-demo/cloud-volume-format-model.json'
        measured = json.loads(evidence.read_text())
        result = volume.capacity(measured['logical_bytes'], measured['free_bytes_after_format'], volume.POOL)
        self.assertEqual(result['surplus_bytes'], measured['surplus_bytes'])
        self.assertLess(measured['physical_bytes'], 2 << 30)

    def test_format_and_mount_stay_scoped_and_do_not_force_or_discard(self):
        m = manifest()
        command = volume.format_command(m, Path('/dev/fixture'))
        self.assertNotIn('-F', command)
        self.assertEqual(command[command.index('-i') + 1], '65536')
        self.assertEqual(command[command.index('-m') + 1], '0')
        self.assertIn('nodiscard,lazy_itable_init=0,lazy_journal_init=0', command)
        unit = volume.mount_unit(m).decode()
        self.assertIn('What=/dev/disk/by-uuid/' + m['filesystem_uuid'], unit)
        self.assertIn('Where=/home/feam-service-data\n', unit)
        self.assertNotIn('nofail', unit)
        self.assertNotIn('ExecStart', unit)
        self.assertEqual(volume.UNIT.name, 'home-feam\\x2dservice\\x2ddata.mount')


if __name__ == '__main__':
    unittest.main()
