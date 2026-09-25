"""Synthetic lifecycle fault injection only; no SSH/service/archive mutations."""
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock

SCRIPTS = Path(__file__).resolve().parents[2] / 'scripts'
sys.path.insert(0, str(SCRIPTS))
spec = importlib.util.spec_from_file_location('site_lifecycle', SCRIPTS / 'site_lifecycle.py')
site = importlib.util.module_from_spec(spec)
spec.loader.exec_module(site)
ID = '00000000-0000-4000-8000-000000000001'
SHA = 'a' * 64


def allocation_document():
    return {'allocation': {'deployment_id': ID, 'project_id': ID, 'run_id': ID, 'activation_generation': 1,
                           'allocation_id': ID, 'amount_usd_micros': 1000, 'expires_at': '2099-01-01T00:00:00Z'}, 'signature': 'synthetic'}


def config():
    doc = allocation_document()
    return {'protocol': 'feam.site-lifecycle.v1', 'project_id': ID, 'deployment_id': ID, 'run_id': ID,
            'activation_generation': 1, 'site_id': 'synthetic-ubuntu', 'actor': 'synthetic-operator', 'workspace_ids': [ID],
            'expected': {'release_sha256': SHA, 'runtime_manifest_sha256': SHA, 'policy_sha256': SHA, 'seed_sha256': SHA,
                         'allocation_id': ID, 'allocation_sha256': hashlib.sha256(json.dumps(doc, separators=(',', ':')).encode()).hexdigest()},
            'ssh': {'host': 'host.example.invalid', 'user': 'feam-deploy', 'port': 22,
                    'identity_file': '/operator/key', 'known_hosts': '/operator/known_hosts'},
            'budget': {'binary': '/operator/project-budget', 'binary_sha256': SHA,
                       'ledger': '/operator/project.enc', 'encryption_key': '/operator/budget.key'},
            'state': {'desired': 'stopped', 'phase': 'ready'}}


def revocations():
    return {'protocol': 'feam.external-revocation.v1', 'deployment_id': ID, 'activation_generation': 1,
            'site_id': 'synthetic-ubuntu', 'route_revoked': True, 'membership_writer_revoked': True,
            'model_credential_revoked': True, 'evidence_sha256': SHA}


class FakeTransport:
    def __init__(self, cfg):
        self.cfg = cfg
        self.calls = []
        self.desired = 'stopped'
        self.run = {'state': 'active', 'document': allocation_document(), 'settled_usd_micros': 0}
        self.additional_runs = {}
        self.fail = None
        self.active = False
        self.archive = {'protocol': 'feam.archive-verification.v1', 'status': 'verified', 'batches': 0,
                        'exports': 0, 'deletions': 0, 'absent': 0, 'bytes': 0,
                        'sha256': hashlib.sha256(b'').hexdigest(), 'verified_at': '2026-09-25T00:00:00Z'}
        self.before_effect = None
        self.ledger = {'protocol': 'feam.archive-ledger.v1', 'sha256': SHA, 'objects': [], 'deletions': [], 'splits': []}

    def call(self, name):
        self.calls.append(name)
        if self.before_effect:
            self.before_effect(name)
        if self.fail == name:
            raise RuntimeError('synthetic lost response')

    def service(self, name, args, timeout=60):
        action = args[1]
        self.call(name + ':' + action)
        if name == 'controller':
            if action == 'status':
                return json.dumps({'protocol': 'feam.lifecycle-status.v1', 'deployment_id': ID,
                    'activation_generation': 1, 'desired': self.desired, 'workspaces': {ID: 'stopped'},
                    'pending_jobs': [], 'pending_snapshots': 0, 'pending_reclaims': 0}).encode()
            self.desired = 'running' if action == 'activate' else 'stopped'
        if name == 'collector' and action == 'archive-verify':
            return json.dumps(self.archive).encode()
        if name == 'collector' and action == 'export-archive-ledger':
            return json.dumps(self.ledger).encode()
        return b''

    def root(self, args, timeout=60):
        self.call('root:' + ' '.join(args))
        if args[0].endswith('sha256sum'):
            key = 'runtime_manifest_sha256' if args[1].endswith('runtime.json') else 'policy_sha256'
            return (self.cfg['expected'][key] + '  private\n').encode()
        if args == ['/usr/bin/cat', '/etc/feam/services/runtime.json']:
            return self.runtime_manifest
        if args[0].endswith('systemctl'):
            if args[1] == 'start':
                self.active = True
            if args[1] == 'disable':
                self.active = False
            if args[1] == 'show':
                return b'ActiveState=inactive\nUnitFileState=disabled\n'
        return b''

    def broker(self, path, post=False):
        self.call('broker:' + path)
        if path == '/v1/status':
            return json.dumps({'usage': {'paused': False, 'allocation_usd_micros': 1000}, 'recording': True}).encode()
        return json.dumps({'protocol': 'feam.broker-drain.v1', 'provenance': 'broker_observed',
                          'allocation_id': ID, 'allocation_sha256': self.cfg['expected']['allocation_sha256'],
                          'deployment_id': ID, 'activation_generation': 1,
                          'usage': {'paused': True, 'reserved_usd_micros': 0, 'settled_usd_micros': 20,
                                    'unknown_usd_micros': 800}, 'unknown_requests': 1}).encode()

    def budget(self, action, *args):
        self.call('budget:' + action)
        if action == 'unknown':
            self.run['state'] = 'unknown'
        if action == 'reconcile':
            self.run.update(state='closed', settled_usd_micros=int(args[args.index('-amount') + 1]),
                            reconciliation_sha256=args[args.index('-evidence-sha256') + 1])
        return json.dumps({'project_id': ID, 'runs': {ID: self.run, **self.additional_runs}}).encode()

    def remote(self, args, timeout=60):
        if args[-1] == '/etc/feam/services/broker/config.json':
            return self.broker_config
        if args[-1] == '/etc/feam/services/collector/config.json':
            return json.dumps({'archive_mode': 'local', 'archive_directory': site.archive_copy.REMOTE_ARCHIVE}).encode()
        self.call('runtime-inventory')
        return b'[]'

    def as_service(self, name, args, timeout=60):
        return self.remote(args, timeout)


class LifecycleTests(unittest.TestCase):
    def engine(self):
        cfg = config()
        transport = FakeTransport(cfg)
        self.snapshots = []
        return site.Lifecycle(cfg, lambda value: self.snapshots.append(copy.deepcopy(value)), transport), transport

    def test_service_execution_drops_from_reviewed_root_sudo(self):
        transport = object.__new__(site.Transport)
        transport.config = config()
        with mock.patch.object(transport, 'root', return_value=b'') as root:
            transport.service('collector', ['-mode', 'flush'])
        args = root.call_args.args[0]
        self.assertEqual(args[:5], ['/usr/sbin/runuser', '-u', 'feam-collector', '--',
                                    '/opt/feam/services/' + SHA + '/collector'])

    def test_start_requires_existing_reserved_run_and_persists_intent_before_effect(self):
        engine, remote = self.engine()
        def check(name):
            if name == 'root:/usr/bin/systemctl enable ' + ' '.join(site.UNITS):
                self.assertEqual(self.snapshots[-1]['state']['desired'], 'running')
                self.assertEqual(self.snapshots[-1]['state']['steps']['enable_services']['state'], 'started')
        remote.before_effect = check
        engine.start()
        self.assertEqual(engine.state['phase'], 'running')
        self.assertNotIn('budget:allocate', remote.calls)
        count = len(remote.calls)
        with self.assertRaises(ValueError):
            engine.start()
        self.assertEqual(len(remote.calls), count)

    def test_missing_or_wrong_budget_blocks_start_before_remote_mutation(self):
        engine, remote = self.engine()
        remote.run['document']['allocation']['deployment_id'] = 'changed'
        with self.assertRaises(ValueError):
            engine.start()
        self.assertEqual(remote.calls, ['budget:status'])
        self.assertFalse(self.snapshots)

    def test_stop_retains_full_unknown_allocation_and_records_archive_before_preparation(self):
        engine, remote = self.engine()
        engine.start()
        engine.stop()
        self.assertEqual(engine.state['desired'], 'stopped')
        self.assertEqual(engine.state['phase'], 'stopped_verified')
        self.assertEqual(remote.run['state'], 'unknown')
        self.assertNotIn('budget:reconcile', remote.calls)
        self.assertEqual(engine.state['receipts']['archive']['inventory'], [])
        self.assertLess(remote.calls.index('broker:/v1/run/drain'), remote.calls.index('collector:flush'))
        receipt = {'protocol': 'feam.mac-archive-copy.v1', 'deployment_id': ID}
        engine.state['mac_archive_copy'] = {'directory': '/operator/copy', 'receipt': receipt}
        with mock.patch.object(site.archive_copy, 'verify_copy', return_value=receipt):
            engine.prepare_teardown(revocations())
        receipt = engine.state['receipts']['teardown']
        self.assertFalse(receipt['destruction_authorized'])
        self.assertEqual(receipt['budget_state'], 'unknown')
        self.assertEqual(remote.calls.count('collector:archive-verify'), 2)
        self.assertFalse(any('destroy' in call or 'rm ' in call for call in remote.calls))

    def test_teardown_requires_fresh_independent_mac_copy(self):
        engine, remote = self.engine()
        engine.start()
        engine.stop()
        with self.assertRaisesRegex(ValueError, 'Mac archive copy'):
            engine.prepare_teardown(revocations())
        receipt = {'protocol': 'feam.mac-archive-copy.v1', 'deployment_id': ID}
        with mock.patch.object(site.archive_copy, 'remote_copy', return_value=receipt), \
             mock.patch.object(site.archive_copy, 'verify_copy', return_value=receipt):
            engine.copy_archive('/operator/research/new-copy')
            engine.prepare_teardown(revocations())
        self.assertEqual(engine.state['phase'], 'teardown_prepared')

    def test_archive_failure_preserves_stopped_intent_and_requires_exact_resume(self):
        engine, remote = self.engine()
        engine.start()
        remote.fail = 'collector:flush'
        with self.assertRaises(RuntimeError):
            engine.stop()
        self.assertEqual(engine.state['desired'], 'stopped')
        self.assertEqual(engine.state['phase'], 'needs_review')
        self.assertEqual(remote.run['state'], 'unknown')
        self.assertNotIn('teardown', engine.state['receipts'])
        with self.assertRaises(ValueError):
            engine.stop()
        with self.assertRaises(ValueError):
            engine.stop('different-operation')
        operation = engine.state['operation_id']
        remote.fail = None
        engine.stop(operation)
        self.assertEqual(engine.state['phase'], 'stopped_verified')
        self.assertEqual(remote.calls.count('broker:/v1/run/drain'), 1)
        self.assertEqual(remote.calls.count('budget:unknown'), 1)

    def test_unknown_start_is_not_replayed_and_can_be_explicitly_stopped(self):
        engine, remote = self.engine()
        remote.fail = 'controller:activate'
        with self.assertRaises(RuntimeError):
            engine.start()
        self.assertEqual(engine.state['phase'], 'needs_review')
        with self.assertRaises(ValueError):
            engine.start()
        remote.fail = None
        engine.stop()
        self.assertEqual(engine.state['desired'], 'stopped')

    def test_stop_intent_survives_unreachable_host_without_allocation_or_retry(self):
        engine, remote = self.engine()
        engine.start()
        remote.fail = 'root:/usr/bin/sha256sum /etc/feam/services/runtime.json'
        with self.assertRaises(RuntimeError):
            engine.stop()
        self.assertEqual(self.snapshots[-1]['state']['desired'], 'stopped')
        self.assertEqual(engine.state['phase'], 'needs_review')
        self.assertEqual(remote.run['state'], 'active')
        self.assertNotIn('budget:allocate', remote.calls)

    def test_missing_revocation_or_changed_archive_blocks_teardown_preparation(self):
        engine, remote = self.engine()
        engine.start()
        engine.stop()
        evidence = revocations()
        evidence['model_credential_revoked'] = False
        with self.assertRaises(ValueError):
            engine.prepare_teardown(evidence)
        remote.archive['sha256'] = SHA
        with self.assertRaises(ValueError):
            engine.prepare_teardown(revocations())
        self.assertNotIn('teardown', engine.state['receipts'])

    def test_failed_reverification_invalidates_old_preparation(self):
        engine, remote = self.engine()
        engine.start()
        engine.stop()
        receipt = {'protocol': 'feam.mac-archive-copy.v1', 'deployment_id': ID}
        engine.state['mac_archive_copy'] = {'directory': '/operator/copy', 'receipt': receipt}
        with mock.patch.object(site.archive_copy, 'verify_copy', return_value=receipt):
            engine.prepare_teardown(revocations())
        remote.fail = 'collector:archive-verify'
        with self.assertRaises(RuntimeError):
            engine.prepare_teardown(revocations())
        self.assertEqual(engine.state['phase'], 'stopped_verified')
        self.assertNotIn('teardown', engine.state['receipts'])

    def test_manual_verified_cost_closure_is_bound_and_not_replayed(self):
        engine, remote = self.engine()
        engine.start()
        engine.stop()
        evidence = {'protocol': 'feam.provider-cost-review.v1', 'allocation_id': ID,
                    'allocation_sha256': engine.config['expected']['allocation_sha256'],
                    'broker_receipt_sha256': site.digest(engine.state['receipts']['broker']),
                    'verified_cost_usd_micros': 30, 'provider_evidence_sha256': SHA}
        engine.reconcile(evidence)
        engine.reconcile(evidence)
        self.assertEqual(remote.calls.count('budget:reconcile'), 1)
        changed = dict(evidence, verified_cost_usd_micros=31)
        with self.assertRaises(ValueError):
            engine.reconcile(changed)
        self.assertEqual(remote.run['settled_usd_micros'], 30)

    def test_closed_input_scope_rejects_shell_targets_and_unknown_command_keys(self):
        for mutate in [lambda c: c['ssh'].update(host='host;touch /tmp/escape'),
                       lambda c: c['ssh'].update(user='root'),
                       lambda c: c['budget'].update(binary='/operator/../bin/tool'),
                       lambda c: c.update(command='destroy')]:
            cfg = config()
            mutate(cfg)
            with self.assertRaises(ValueError):
                site.validate(cfg)

    def test_prior_archive_lineage_and_prior_site_revocation_block_fresh_activation(self):
        engine, remote = self.engine()
        old = copy.deepcopy(remote.run)
        old['document']['allocation']['deployment_id'] = '00000000-0000-4000-8000-000000000099'
        remote.additional_runs['previous'] = old
        with self.assertRaisesRegex(ValueError, 'archive baseline'):
            engine.start()
        engine.state['archive_ledger'] = copy.deepcopy(remote.ledger)
        with self.assertRaisesRegex(ValueError, 'revocation evidence'):
            engine.start()
        self.assertFalse(self.snapshots)
        engine.state['archive_ledger']['deletions'] = [{'participant_id': ID}]
        with self.assertRaisesRegex(ValueError, 'withdrawal lineage'):
            engine.start()

    def test_next_run_proposal_preserves_old_ledgers_and_requires_reserved_new_ids(self):
        engine, remote = self.engine()
        engine.start()
        engine.stop()
        copy_record = {'directory': '/operator/research/old-copy',
                       'receipt': {'protocol': 'feam.mac-archive-copy.v1', 'deployment_id': ID}}
        engine.state['mac_archive_copy'] = copy.deepcopy(copy_record)
        engine.state['mac_archive_copies'] = [copy.deepcopy(copy_record)]
        old_root = '/home/feam-service-data/operations/broker/'
        remote.broker_config = json.dumps({'ledger': old_root + 'old.sqlite', 'capabilities': old_root + 'old-caps.sqlite',
            'receipt_file': old_root + 'old-receipt.json', 'allocation': '/etc/feam/services/broker/allocation.json'}).encode()
        manifest = {'release_sha256': SHA, 'workspace_ids': [ID], 'mounts': {'operations': ID, 'traces': ID, 'datasets': ID},
                    'config_sha256': {name: SHA for name in site.SERVICES}}
        manifest['config_sha256']['broker'] = hashlib.sha256(remote.broker_config).hexdigest()
        remote.runtime_manifest = json.dumps(manifest).encode()
        engine.config['expected']['runtime_manifest_sha256'] = hashlib.sha256(remote.runtime_manifest).hexdigest()
        variables = {'feam_services_release_sha256': SHA, 'feam_workspace_ids': [ID], 'feam_service_mount_uuids': manifest['mounts'],
                     'feam_service_configs': {name: {'source': '/operator/' + name, 'sha256': value}
                                             for name, value in manifest['config_sha256'].items()}}
        document = allocation_document()
        document['allocation'].update(allocation_id='00000000-0000-4000-8000-000000000002', run_id='00000000-0000-4000-8000-000000000003')
        remote.additional_runs[document['allocation']['allocation_id']] = {'state': 'active', 'document': document}
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary).resolve()
            os.chmod(directory, 0o700)
            record = {'other_namespace': 'preserved', 'site_lifecycle': engine.config}
            receipt = engine.prepare_next_run(document, variables, directory, record)
            self.assertFalse(receipt['host_changed'])
            self.assertFalse(receipt['allocation_issued'])
            self.assertEqual(engine.state['phase'], 'stopped_verified')
            proposal = json.loads((directory / 'operator-next.json').read_text())
            self.assertEqual(proposal['other_namespace'], 'preserved')
            self.assertEqual(proposal['site_lifecycle']['state']['phase'], 'ready')
            self.assertEqual(proposal['site_lifecycle']['state']['mac_archive_copy'], copy_record)
            self.assertEqual(proposal['site_lifecycle']['state']['mac_archive_copies'], [copy_record])
            broker = json.loads((directory / 'broker.json').read_text())
            self.assertNotEqual(broker['ledger'], old_root + 'old.sqlite')
            self.assertIn(document['allocation']['run_id'], broker['ledger'])
            prepared = json.loads((directory / 'service-vars.json').read_text())
            self.assertEqual(prepared['feam_services_initialize'], ['broker'])
            self.assertFalse(prepared['feam_services_start'])
            with mock.patch.dict(os.environ, {'ANSIBLE_HOME': str(directory / 'ansible'),
                                               'ANSIBLE_LOCAL_TEMP': str(directory / 'ansible/tmp')}):
                from ansible.plugins.filter.core import to_nice_json
                actual_manifest = json.loads((directory / 'runtime-manifest.json').read_text())
                self.assertEqual((directory / 'runtime-manifest.json').read_bytes(), to_nice_json(actual_manifest).encode())
            self.assertNotIn('budget:allocate', remote.calls)
            self.assertEqual(remote.run['state'], 'unknown')

    def test_status_reads_encrypted_vault_without_network_or_secret_output(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary).resolve()
            os.chmod(directory, 0o700)
            key = directory / 'key'
            source = directory / 'source.json'
            target = directory / 'operator.enc'
            site.vault.keygen(key)
            site.vault.new_private(source, json.dumps({'unrelated': {'secret': 'secret-not-output'}, 'site_lifecycle': config()}).encode())
            site.vault.seal(source, target, key, 0)
            result = subprocess.run([sys.executable, str(SCRIPTS / 'site_lifecycle.py'), 'status', '--vault', str(target),
                                     '--key', str(key), '--expected-revision', '1'], capture_output=True, check=True)
            output = json.loads(result.stdout)
            self.assertEqual(output['phase'], 'ready')
            self.assertNotIn(b'secret-not-output', result.stdout)
            self.assertNotIn(b'host.example', target.read_bytes())
            wrong = subprocess.run([sys.executable, str(SCRIPTS / 'site_lifecycle.py'), 'status', '--vault', str(target),
                                    '--key', str(key), '--expected-revision', '2'], capture_output=True)
            self.assertNotEqual(wrong.returncode, 0)

    def test_operator_subprocess_output_and_deadline_are_bounded(self):
        transport = site.Transport.__new__(site.Transport)
        transport.deadline = time.monotonic() + 10
        with self.assertRaisesRegex(ValueError, 'exceeded bound'):
            transport.command([sys.executable, '-c', 'import sys; sys.stdout.buffer.write(b"x" * 1100000)'])
        with self.assertRaisesRegex(ValueError, 'deadline'):
            transport.command([sys.executable, '-c', 'import time; time.sleep(2)'], timeout=0.05)


if __name__ == '__main__':
    unittest.main()
