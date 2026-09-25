"""U.04 rendering must bind staged owners and remain inert until activation."""
import base64
import hashlib
import json
from pathlib import Path
import sys
import tempfile
import unittest
import uuid
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'scripts'))
import services_runtime
import u04_service_configs as render


IDS = ('2507d292-7be1-4152-99f5-a685f15fa3e8',
       'b817e483-bfdb-4bb6-8b52-a9504a775895')
OWNERS = ('f922d548-6f0f-45dd-86d5-bd93259efdd2',
          'fa6baa0c-6b0f-4f28-b756-3bb0a8e0e77d')
PRELIMINARY = ('9e5203d1-46b0-47b0-ace6-e67d88550f48',
               'd071f0b4-6e43-497e-947a-7967a9dc9f15')
DEPLOYMENT = '19c62e83-6b59-41dc-848d-bb80c5a024b0'
RUN = 'c9a6d480-9fa5-4328-b9f0-5b2060cc23ab'
ALLOCATION = '042ebaf6-a138-4db9-b4d0-3dbc6f3757a0'


class RenderTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.root.chmod(0o700)
        self.profile = self.root / 'agent.toml'
        self.profile.write_bytes(b'[agent]\nmodel="synthetic"\n')
        self.profile.chmod(0o600)
        self.signed = self.root / 'allocation.json'
        self.signed.write_text(json.dumps({'allocation': {'deployment_id': DEPLOYMENT,
            'activation_generation': 1, 'run_id': RUN, 'allocation_id': ALLOCATION}}))
        self.signed.chmod(0o600)
        self.pub = self.root / 'operator.pub'
        self.pub.write_bytes(b'P' * 32)
        self.pub.chmod(0o600)
        self.token = self.root / 'token'
        self.token.write_bytes(b'synthetic-test-token')
        self.token.chmod(0o600)
        self.topology = {'deployment_id': DEPLOYMENT, 'activation_generation': 1,
            'workspace_ids': list(IDS), 'reviewer_uid': 1000,
            'native_go_source_sha256': 'a' * 64,
            'native_go_artifact_sha256': {name: 'b' * 64 for name in render.SERVICES},
            'mount_uuids': {name: DEPLOYMENT for name in ('operations', 'traces', 'datasets')},
            'slots': [{'id': f'slot-{i:02}', 'path': f'/home/feam-service-data/slot-{i:02}',
                       'filesystem_uuid': DEPLOYMENT, 'terminal_path': ''} for i in range(3, 7)]}
        hosts = ['feam.613202690.xyz', 'admin.613202690.xyz',
                 *[f'u-{wid}.613202690.xyz' for wid in IDS]]
        self.inputs = {'phase': 'final', 'release_sha256': 'a' * 64, 'frontend_uid': 2109,
            'artifact_sources': {name: f'/approved/{name}' for name in render.ARTIFACTS},
            'feam_sha256': 'c' * 64, 'image_id': 'sha256:' + 'd' * 64,
            'dataset_sha256': 'e' * 64,
            'profile_sha256': hashlib.sha256(self.profile.read_bytes()).hexdigest(),
            'profile_source': str(self.profile), 'fake_allocation_source': str(self.signed),
            'fake_public_key_source': str(self.pub), 'fake_run_id': RUN,
            'fake_allocation_id': ALLOCATION,
            'cloudflare': {'issuer': 'https://test.cloudflareaccess.com',
                'account_id': 'f' * 32, 'user_group': OWNERS[0], 'admin_group': OWNERS[1],
                'group_names': {OWNERS[0]: 'test-users', OWNERS[1]: 'test-admins'},
                'audiences': {host: f'{i:064x}' for i, host in enumerate(hosts, 1)},
                'token_source': str(self.token)},
            'owner_ids': list(OWNERS), 'preliminary_owner_ids': list(PRELIMINARY),
            'participant_ids': list(PRELIMINARY), 'reviewer': 'test-reviewer',
            'consent_reference': 'synthetic-test-consent'}

    def test_final_config_is_private_scoped_and_inert(self):
        destination = self.root / 'out'
        receipt = render.render(self.inputs, self.topology, destination)
        variables = json.loads((destination / 'service-vars.json').read_text())
        manifest = {'protocol': 'feam.services.v1', 'release_sha256': receipt['release_sha256'],
            'frontend_uid': variables['feam_gateway_frontend_uid'],
            'mounts': variables['feam_service_mount_uuids'],
            'workspace_ids': variables['feam_workspace_ids'],
            'config_sha256': receipt['config_sha256'],
            'artifact_sha256': {name: value['sha256'] for name, value in variables['feam_service_artifacts'].items()},
            'broker_mode': variables['feam_services_broker_mode']}
        services_runtime.validate_manifest(manifest)
        self.assertEqual(set(manifest['artifact_sha256']), set(render.ARTIFACTS))
        self.assertFalse(variables['feam_services_start'])
        self.assertFalse(variables['feam_services_enable'])
        self.assertEqual(variables['feam_services_initialize'], [])
        self.assertEqual(variables['feam_services_controller_slots']['/home/feam-service-data/slot-03'], DEPLOYMENT)
        controller = json.loads((destination / 'controller.json').read_text())
        self.assertEqual([w['owner_id'] for w in controller['workspaces']], list(OWNERS))
        collector = json.loads((destination / 'collector.json').read_text())
        self.assertEqual(collector['archive_mode'], 'local')
        self.assertEqual(collector['r2_account_id'], '')
        self.assertEqual(len(json.loads((destination / 'collector-policy.json').read_text())), 2)
        self.assertEqual(json.loads((destination / 'broker.json').read_text())['upstream_key_file'], '')
        self.assertEqual((destination / 'reconciler-token').stat().st_mode & 0o077, 0)

    def test_seven_member_roster_has_exact_endpoints_and_policies(self):
        for index in range(5):
            wid = str(uuid.uuid4())
            self.topology['workspace_ids'].append(wid)
            self.inputs['owner_ids'].append(str(uuid.uuid4()))
            self.inputs['participant_ids'].append(str(uuid.uuid4()))
            self.inputs['cloudflare']['audiences']['u-' + wid + '.613202690.xyz'] = f'{index + 5:064x}'
        out = self.root / 'cohort'
        render.render(self.inputs, self.topology, out)
        controller = json.loads((out / 'controller.json').read_text())
        broker = json.loads((out / 'broker.json').read_text())
        collector = json.loads((out / 'collector.json').read_text())
        self.assertEqual(len(controller['workspaces']), 7)
        self.assertEqual(set(controller['endpoints']), set(self.topology['workspace_ids']))
        self.assertEqual({v['workspace_id'] for v in broker['sockets']}, set(controller['endpoints']))
        self.assertEqual(set(collector['workspace_sockets']), set(controller['endpoints']))
        self.assertEqual(len(json.loads((out / 'collector-policy.json').read_text())), 7)

    def test_mismatched_roster_cannot_silently_truncate_zip(self):
        self.inputs['participant_ids'].append(str(uuid.uuid4()))
        with self.assertRaisesRegex(ValueError, 'participant per workspace'):
            render.render(self.inputs, self.topology, self.root / 'out')
        self.assertFalse((self.root / 'out').exists())

    def test_final_rejects_even_one_provisional_owner(self):
        self.inputs['owner_ids'][1] = PRELIMINARY[1]
        with self.assertRaisesRegex(ValueError, 'every owner'):
            render.render(self.inputs, self.topology, self.root / 'out')

    def test_duplicate_audience_rejected_before_writing(self):
        self.inputs['cloudflare']['audiences']['admin.613202690.xyz'] = '1'.zfill(64)
        with self.assertRaisesRegex(ValueError, 'audiences'):
            render.render(self.inputs, self.topology, self.root / 'out')
        self.assertFalse((self.root / 'out').exists())

    def test_live_transition_uses_separate_run_and_private_key(self):
        live_run = '470d4fd4-b385-4ce3-a22e-972c601c98a7'
        live_allocation = '514c68da-301f-4c69-9c9d-35a8cb2b309c'
        live_project = 'a3e8316a-d5c7-4f71-8a4b-96429d704629'
        live_signed = self.root / 'live-allocation.json'
        allocation = dict(protocol='feam.web.v1', allocation_id=live_allocation,
            project_id=live_project, run_id=live_run, deployment_id=DEPLOYMENT,
            activation_generation=1, ledger_revision=2, amount_usd_micros=1000000,
            request_limit_usd_micros=30000, user_daily_limit_usd_micros=100000,
            model='deepseek/deepseek-v4.1-flash', provider='deepinfra/fp8',
            profile='phase1-demo', input_price_usd_micros_per_million=100000,
            output_price_usd_micros_per_million=100000, fee_basis_points=10000,
            max_output_tokens=128, not_before='2026-09-25T00:00:00Z',
            expires_at='2026-09-27T00:00:00Z')
        signer = Ed25519PrivateKey.generate()
        live_pub = self.root / 'live-operator.pub'
        live_pub.write_bytes(signer.public_key().public_bytes(
            serialization.Encoding.Raw, serialization.PublicFormat.Raw))
        live_pub.chmod(0o600)
        signed_bytes = json.dumps({name: allocation[name] for name in render.ALLOCATION_FIELDS},
                                  separators=(',', ':')).encode()
        live_signed.write_text(json.dumps({'allocation': allocation,
            'signature': base64.b64encode(signer.sign(signed_bytes)).decode()}))
        live_signed.chmod(0o600)
        key = self.root / 'upstream.key'
        key.write_bytes(b'private-live-test-key\n')
        key.chmod(0o600)
        self.inputs.update(broker_mode='live', live_project_id=live_project,
            live_run_id=live_run, live_allocation_id=live_allocation,
            live_allocation_source=str(live_signed), live_public_key_source=str(live_pub),
            upstream_key_source=str(key), previous_fake_run_id=RUN,
            previous_fake_allocation_id=ALLOCATION,
            previous_fake_allocation_source=str(self.signed),
            previous_fake_public_key_source=str(self.pub))
        output = self.root / 'live'
        receipt = render.render(self.inputs, self.topology, output)
        variables = json.loads((output / 'service-vars.json').read_text())
        broker = json.loads((output / 'broker.json').read_text())
        controller = json.loads((output / 'controller.json').read_text())
        policy = json.loads((output / 'collector-policy.json').read_text())
        self.assertEqual(receipt['broker_mode'], 'live')
        self.assertEqual(variables['feam_services_broker_mode'], 'live')
        self.assertEqual(variables['feam_services_initialize'], ['broker'])
        repeat = json.loads((output / 'service-vars-post-init.json').read_text())
        self.assertEqual(repeat['feam_services_initialize'], [])
        self.assertEqual(repeat['feam_service_configs'], variables['feam_service_configs'])
        self.assertIn(live_run, broker['ledger'])
        self.assertEqual(broker['upstream_key_file'], '/etc/feam/services/broker/upstream.key')
        self.assertEqual((output / 'broker-upstream.key').read_bytes(), key.read_bytes())
        self.assertEqual((output / 'previous-fake-allocation.json').read_bytes(), self.signed.read_bytes())
        self.assertEqual(receipt['preserved_fake_public_key_sha256'], hashlib.sha256(self.pub.read_bytes()).hexdigest())
        self.assertTrue(all(not v['synthetic'] for v in controller['endpoints'].values()))
        self.assertTrue(all(not v['synthetic'] for v in policy))
        self.assertTrue(all(v['consent_reference'] == self.inputs['consent_reference'] for v in policy))
        self.assertNotIn('fake_allocation_sha256', receipt)
        self.assertFalse(variables['feam_services_start'])
        forged = json.loads(live_signed.read_text())
        forged['allocation']['amount_usd_micros'] += 1
        live_signed.write_text(json.dumps(forged))
        with self.assertRaisesRegex(ValueError, 'verification failed'):
            render.render(self.inputs, self.topology, self.root / 'forged')
        self.assertFalse((self.root / 'forged').exists())

    def test_live_transition_rejects_fake_allocation_or_missing_key(self):
        key = self.root / 'upstream.key'
        key.write_bytes(b'private-live-test-key\n')
        key.chmod(0o600)
        self.inputs.update(broker_mode='live', live_project_id=DEPLOYMENT,
            live_run_id=RUN, live_allocation_id=ALLOCATION,
            live_allocation_source=str(self.signed), live_public_key_source=str(self.pub),
            upstream_key_source=str(key), previous_fake_run_id=RUN,
            previous_fake_allocation_id=ALLOCATION,
            previous_fake_allocation_source=str(self.signed),
            previous_fake_public_key_source=str(self.pub))
        with self.assertRaisesRegex(ValueError, 'must differ'):
            render.render(self.inputs, self.topology, self.root / 'live')
        self.assertFalse((self.root / 'live').exists())
        self.inputs['live_run_id'] = '470d4fd4-b385-4ce3-a22e-972c601c98a7'
        self.inputs['live_allocation_id'] = '514c68da-301f-4c69-9c9d-35a8cb2b309c'
        with self.assertRaisesRegex(ValueError, 'signed allocation binding'):
            render.render(self.inputs, self.topology, self.root / 'live')
        self.inputs['upstream_key_source'] = str(self.root / 'missing-key')
        with self.assertRaises(FileNotFoundError):
            render.render(self.inputs, self.topology, self.root / 'live')


if __name__ == '__main__':
    unittest.main()
