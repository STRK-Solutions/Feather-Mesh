"""Offline service scope checks; does not claim systemd/ACL target acceptance."""
import copy
import importlib.util
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[4]
spec = importlib.util.spec_from_file_location('services_runtime', ROOT / 'infra/demo/scripts/services_runtime.py')
runtime = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runtime)
ID = '00000000-0000-4000-8000-000000000001'
SHA = 'a' * 64


def manifest():
    return {'protocol': 'feam.services.v1', 'release_sha256': SHA, 'frontend_uid': 2999,
            'mounts': {key: ID for key in ('operations', 'traces', 'datasets')},
            'workspace_ids': [ID], 'config_sha256': {key: SHA for key in runtime.UIDS},
            'artifact_sha256': {key: SHA for key in [*runtime.UIDS, 'feam']}, 'broker_mode': 'fake'}


def configs():
    base = '/run/feam/services/'
    operator = '/etc/feam/services/'
    state = '/home/feam-service-data/'
    return {
        'gateway': {'db': state + 'operations/gateway/control.db', 'browser_socket': base + 'gateway/browser.sock',
                    'control_socket': base + 'gateway/control.sock', 'controller_uid': 2103, 'reconciler_uid': 2107, 'pipeline_uid': 2106,
                    'read_uids': [2103, 2104, 2105, 2106, 2107],
                    'gateway': {'controller_socket': base + 'controller/control.sock', 'pipeline_socket': base + 'pipeline/control.sock',
                                'broker_socket': base + 'broker/control.sock'}},
        'controller': {'database': state + 'operations/controller/lifecycle.db', 'socket': base + 'controller/control.sock',
                       'gateway_socket': base + 'gateway/control.sock', 'runtime_socket': '/run/user/2101/feam-docker.sock',
                       'gateway_uid': 2102, 'broker_uid': 2104,
                       'broker_socket': base + 'broker/control.sock', 'collector_socket': base + 'collector/control.sock',
                       'pipeline_socket': base + 'pipeline/control.sock', 'release_root': state + 'datasets/pipeline/releases',
                       'profile_path': operator + 'controller/agent.toml',
                       'endpoints': {ID: {'terminal_directory': '/run/feam/workspaces/' + ID + '/terminal',
                                          'model_directory': '/run/feam/workspaces/' + ID + '/model',
                                          'event_directory': '/run/feam/workspaces/' + ID + '/events',
                                          'config_directory': '/run/feam/workspaces/' + ID + '/config',
                                          'participant_id': ID, 'synthetic': True,
                                          'software_sha256': SHA, 'profile_sha256': SHA, 'dataset_sha256': SHA}}},
        'broker': {'ledger': state + 'operations/broker/run.db', 'capabilities': state + 'operations/broker/capabilities.db',
                   'control_socket': base + 'broker/control.sock', 'authority_socket': base + 'gateway/control.sock',
                   'controller_uid': 2103, 'gateway_uid': 2102, 'collector_socket': base + 'collector/control.sock',
                   'controller_socket': base + 'controller/control.sock',
                   'receipt_file': state + 'operations/broker/drain-receipt.json',
                   'sockets': [{'workspace_id': ID, 'path': '/run/feam/workspaces/' + ID + '/model/broker.sock'}],
                   'allocation': operator + 'broker/allocation.json', 'operator_public_key': operator + 'broker/operator.pub'},
        'collector': {'db': state + 'traces/collector/events.db', 'capabilities_db': state + 'traces/collector/capabilities.db',
                      'control_socket': base + 'collector/control.sock', 'gateway_socket': base + 'gateway/control.sock',
                      'broker_uid': 2104, 'controller_uid': 2103, 'verifier_uid': 2106, 'reviewer_uids': [2998],
                      'reviewer': 'approved-research-reviewer',
                      'workspace_sockets': {ID: '/run/feam/workspaces/' + ID + '/events/events.sock'},
                      'policy_file': operator + 'collector/policy.json', 'archive_mode': 'local',
                      'archive_directory': state + 'traces/collector/archive', 'local_max_bytes': 20 << 30,
                      'archive_max_bytes': 1 << 30},
        'pipeline': {'root': state + 'datasets/pipeline', 'feam': '/opt/feam/services/' + SHA + '/feam',
                     'python': '/opt/feam/pipeline/bin/python', 'converter': '/opt/feam/importers/convert.py',
                     'socket': base + 'pipeline/control.sock', 'controller_uid': 2103, 'gateway_uid': 2102,
                     'control_socket': base + 'gateway/control.sock'},
        'reconciler': {'control_socket': base + 'gateway/control.sock', 'token_file': operator + 'reconciler/token'},
    }


class ServicesTests(unittest.TestCase):
    def test_fixed_manifest_scope_and_closed_fields(self):
        runtime.validate_manifest(manifest())
        for field, value in [('frontend_uid', 200999), ('frontend_uid', 2104), ('broker_mode', 'fallback'),
                             ('release_sha256', '../outside'), ('workspace_ids', [ID, ID])]:
            bad = manifest()
            bad[field] = value
            with self.assertRaises(ValueError):
                runtime.validate_manifest(bad)
        bad = manifest()
        bad['command'] = 'shell'
        with self.assertRaises(ValueError):
            runtime.validate_manifest(bad)

    def test_each_configuration_has_separate_state_and_authority(self):
        for service, value in configs().items():
            runtime.validate_config(service, value, manifest())
        for service, field, bad_value in [
            ('gateway', 'db', '/home/saif/private.db'),
            ('broker', 'ledger', '/home/feam-service-data/operations/gateway/control.db'),
            ('broker', 'ledger', '/home/feam-service-data/operations/broker/../gateway/control.db'),
            ('broker', 'operator_public_key', '/etc/feam/services/collector/signing.key'),
            ('controller', 'runtime_socket', '/var/run/docker.sock'),
            ('controller', 'profile_path', '/etc/feam/services/broker/operator.pub'),
            ('collector', 'verifier_uid', 2104),
            ('collector', 'reviewer_uids', []),
            ('collector', 'reviewer_uids', [2104]),
            ('collector', 'reviewer_uids', [2999]),
            ('collector', 'reviewer_uids', [200999]),
            ('collector', 'archive_mode', 'fixture'),
            ('collector', 'archive_directory', '/tmp/archive'),
            ('reconciler', 'token_file', '/home/saif/token'),
            ('pipeline', 'python', '/bin/sh'),
        ]:
            config = configs()[service]
            config[field] = bad_value
            with self.assertRaises(ValueError, msg=service + ':' + field):
                runtime.validate_config(service, config, manifest())

    def test_collector_archive_modes_are_explicit_and_disjoint(self):
        local = configs()['collector']
        local['r2_bucket'] = 'unexpected'
        with self.assertRaises(ValueError):
            runtime.validate_config('collector', local, manifest())
        r2 = configs()['collector']
        r2.update(archive_mode='r2', archive_directory='',
                  r2_credentials_file='/etc/feam/services/collector/r2.json',
                  r2_account_id='test-account', r2_bucket='synthetic-test-only')
        runtime.validate_config('collector', r2, manifest())
        r2['r2_credentials_file'] = '/etc/feam/services/gateway/r2.json'
        with self.assertRaises(ValueError):
            runtime.validate_config('collector', r2, manifest())

    def test_model_endpoint_is_exact_workspace_and_live_requires_own_credential(self):
        config = configs()['broker']
        config['sockets'][0]['path'] = '/run/feam/workspaces/' + ID + '/events/events.sock'
        with self.assertRaises(ValueError):
            runtime.validate_config('broker', config, manifest())
        value = manifest()
        value['broker_mode'] = 'live'
        config = configs()['broker']
        config['upstream_key_file'] = '/etc/feam/services/gateway/upstream'
        with self.assertRaises(ValueError):
            runtime.validate_config('broker', config, value)
        config['upstream_key_file'] = '/etc/feam/services/broker/upstream'
        runtime.validate_config('broker', config, value)

    def test_hardened_template_and_explicit_initialization(self):
        import jinja2
        import yaml
        role = ROOT / 'infra/demo/ansible/roles/services'
        variables = yaml.safe_load((role / 'vars/main.yml').read_text())
        context = dict(variables, feam_services_release_sha256=SHA, feam_services_broker_mode='fake', feam_workspace_ids=[ID])
        env = jinja2.Environment(undefined=jinja2.StrictUndefined)
        context['services_commands'] = {name: env.from_string(cmd).render(context) for name, cmd in variables['services_commands'].items()}
        template = env.from_string((role / 'templates/participant.service.j2').read_text())
        for name in runtime.UIDS:
            unit = template.render(context, item=name)
            for setting in ('User=feam-' + name, 'ProtectSystem=strict', 'NoNewPrivileges=yes', 'MemorySwapMax=0',
                            'CapabilityBoundingSet=', 'run-feam.mount', 'prepare --service ' + name):
                self.assertIn(setting, unit)
            self.assertNotIn('-initialize', unit)
            self.assertNotIn('-mode migrate', unit)
            if name == 'controller':
                self.assertIn('RestrictAddressFamilies=AF_UNIX\n', unit)
                self.assertNotIn('/var/run/docker.sock', unit)
            if name == 'pipeline':
                self.assertIn('--web-demo-acl', unit)
                self.assertIn('Slice=feam-pipeline.slice', unit)
            else:
                self.assertIn('Slice=feam-control.slice', unit)
        defaults = yaml.safe_load((ROOT / 'infra/demo/ansible/services.yml').read_text())[0]['vars']
        self.assertFalse(defaults['feam_services_start'])
        self.assertFalse(defaults['feam_services_enable'])
        self.assertEqual(defaults['feam_services_initialize'], [])
        self.assertEqual(defaults['feam_services_migrate'], [])
        self.assertEqual(defaults['feam_services_broker_mode'], 'fake')

    def test_cross_service_policy_hash_is_checked_before_root_acl_mutation(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / 'collector/config.json'
            path.parent.mkdir()
            value = configs()['collector']
            raw = json.dumps(value).encode()
            path.write_bytes(raw)
            policy = manifest()
            policy['config_sha256']['collector'] = hashlib.sha256(raw).hexdigest()
            with mock.patch.object(runtime, 'CONFIG', root), mock.patch.object(runtime, 'read_json', return_value=value), \
                    mock.patch.object(runtime, 'directory') as create:
                # Hash validation runs before interpreting another service's
                # policy, regardless of whether its semantic content is valid.
                path.write_bytes(raw + b' ')
                with self.assertRaisesRegex(ValueError, 'config revision mismatch'):
                    runtime.layout('gateway', policy)
                create.assert_not_called()

    def test_health_preserves_intentionally_inactive_services(self):
        with mock.patch.object(runtime.subprocess, 'check_output', return_value='inactive\n') as command, \
                mock.patch.object(runtime, 'socket_specs') as sockets:
            runtime.health('broker', manifest())
            sockets.assert_not_called()
            self.assertEqual(command.call_args.args[0],
                             ['systemctl', 'show', 'feam-broker.service', '--property=ActiveState', '--value'])
        with mock.patch.object(runtime.subprocess, 'check_output', return_value='failed\n'):
            with self.assertRaises(ValueError):
                runtime.health('broker', manifest())

    def test_manifest_converge_does_not_restart_runner_dependency(self):
        import yaml
        tasks = yaml.safe_load((ROOT / 'infra/demo/ansible/roles/services/tasks/participant.yml').read_text())
        layout = next(i for i, task in enumerate(tasks) if task['name'] ==
                      'Verify current owned inputs and provision fixed runtime directories')
        runtime_unit = next(i for i, task in enumerate(tasks) if task['name'] ==
                            'Keep fixed boot layout active without cycling the runner manager')
        self.assertLess(layout, runtime_unit)
        self.assertEqual(tasks[layout]['ansible.builtin.command']['argv'],
                         ['/usr/local/sbin/feam-services-runtime', 'layout', '--service', '{{ item }}'])
        self.assertEqual(tasks[runtime_unit]['ansible.builtin.systemd_service']['name'],
                         'feam-service-runtime.service')
        self.assertEqual(tasks[runtime_unit]['ansible.builtin.systemd_service']['state'], 'started')


if __name__ == '__main__':
    unittest.main()
