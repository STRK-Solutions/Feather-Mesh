#!/usr/bin/env python3
"""Explicit private site lifecycle; never creates compute, deletes storage or issues allocations."""
import argparse
import copy
from datetime import datetime, timezone
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import selectors
import shlex
import signal
import stat
import subprocess
import tempfile
import time
import uuid

import operator_vault as vault
import archive_copy

SERVICES = ('gateway', 'pipeline', 'collector', 'broker', 'reconciler', 'controller')
UNITS = tuple('feam-' + name + '.service' for name in SERVICES)
HEX = re.compile(r'^[0-9a-f]{64}$')
UUID = re.compile(r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
LIMIT = 1 << 20


def require(condition, message):
    if not condition:
        raise ValueError(message)


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False).encode()


def digest(value):
    return hashlib.sha256(canonical(value)).hexdigest()


def timestamp():
    return datetime.now(timezone.utc).isoformat()


def validate(config):
    require(set(config) == {'protocol', 'project_id', 'deployment_id', 'run_id', 'activation_generation',
                           'site_id', 'actor', 'workspace_ids', 'expected', 'ssh', 'budget', 'state'},
            'closed lifecycle record required')
    require(config['protocol'] == 'feam.site-lifecycle.v1', 'unsupported lifecycle protocol')
    for name in ('project_id', 'deployment_id', 'run_id'):
        require(UUID.fullmatch(config[name]), 'opaque identity required')
    require(type(config['activation_generation']) is int and config['activation_generation'] > 0 and
            re.fullmatch(r'[a-z][a-z0-9-]{0,62}', config['site_id']) and
            isinstance(config['actor'], str) and 0 < len(config['actor']) <= 128, 'invalid activation attribution')
    require(1 <= len(config['workspace_ids']) <= 10 and len(set(config['workspace_ids'])) == len(config['workspace_ids']) and
            all(UUID.fullmatch(v) for v in config['workspace_ids']), 'fixed workspace identities required')
    expected = config['expected']
    require(set(expected) == {'release_sha256', 'runtime_manifest_sha256', 'policy_sha256', 'seed_sha256',
                             'allocation_id', 'allocation_sha256'}, 'reviewed release/policy/seed/allocation required')
    require(UUID.fullmatch(expected['allocation_id']) and all(HEX.fullmatch(v) for k, v in expected.items() if k != 'allocation_id'),
            'invalid review digest')
    ssh = config['ssh']
    require(set(ssh) == {'host', 'user', 'port', 'identity_file', 'known_hosts'} and ssh['user'] == 'feam-deploy' and
            re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9.-]{0,252}', ssh['host']) and type(ssh['port']) is int and
            1 <= ssh['port'] <= 65535, 'verified dedicated SSH target required')
    budget = config['budget']
    require(set(budget) == {'binary', 'binary_sha256', 'ledger', 'encryption_key'} and HEX.fullmatch(budget['binary_sha256']),
            'pinned durable operator ledger required')
    for path in [ssh['identity_file'], ssh['known_hosts'], budget['binary'], budget['ledger'], budget['encryption_key']]:
        require(isinstance(path, str) and Path(path).is_absolute() and str(Path(path)) == path and '..' not in Path(path).parts,
                'absolute private file references required')
    state = config['state']
    require(isinstance(state, dict) and state['desired'] in ('running', 'stopped') and
            state['phase'] in ('ready', 'starting', 'running', 'stopping', 'needs_review', 'stopped_verified', 'teardown_prepared'),
            'explicit lifecycle state required')
    return config


class Transport:
    """Bounded output and deadlines; no local shell and only fixed remote argv."""
    def __init__(self, config):
        self.config = config
        self.deadline = time.monotonic() + 600
        for name in ('identity_file', 'known_hosts'):
            vault.private_read(config['ssh'][name])
        binary = Path(config['budget']['binary'])
        info = binary.lstat()
        require(stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid() and not info.st_mode & 0o022 and info.st_size <= 128 << 20,
                'reviewed operator binary ownership mismatch')
        require(hashlib.sha256(binary.read_bytes()).hexdigest() == config['budget']['binary_sha256'], 'operator binary changed')

    def command(self, argv, timeout=60):
        deadline = min(self.deadline, time.monotonic() + timeout)
        require(deadline > time.monotonic(), 'operation deadline exhausted')
        proc = subprocess.Popen(argv, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                stderr=subprocess.DEVNULL, start_new_session=True)
        output = bytearray()
        try:
            with selectors.DefaultSelector() as selector:
                selector.register(proc.stdout, selectors.EVENT_READ)
                while selector.get_map():
                    remaining = deadline - time.monotonic()
                    require(remaining > 0, 'command outcome unknown after deadline')
                    for key, _ in selector.select(min(remaining, 1)):
                        part = os.read(key.fileobj.fileno(), 65536)
                        if not part:
                            selector.unregister(key.fileobj)
                        output.extend(part)
                        require(len(output) <= LIMIT, 'private result exceeded bound')
            require(proc.wait(timeout=max(.01, deadline-time.monotonic())) == 0,
                    'command failed; inspect private service state before resuming')
            return bytes(output)
        finally:
            if proc.poll() is None:
                os.killpg(proc.pid, signal.SIGKILL)
                proc.wait()
            proc.stdout.close()

    def remote(self, argv, timeout=60):
        cfg = self.config['ssh']
        return self.command(['/usr/bin/ssh', '-T', '-oBatchMode=yes', '-oIdentitiesOnly=yes',
                             '-oStrictHostKeyChecking=yes', '-oConnectTimeout=10', '-oServerAliveInterval=10',
                             '-oServerAliveCountMax=2', '-oUserKnownHostsFile=' + cfg['known_hosts'],
                             '-i', cfg['identity_file'], '-p', str(cfg['port']), '--', cfg['user'] + '@' + cfg['host'],
                             shlex.join(argv)], timeout)

    def root(self, argv, timeout=60):
        return self.remote(['sudo', '-n', '--', *argv], timeout)

    def as_service(self, name, argv, timeout=60):
        require(name in SERVICES, 'unknown fixed service')
        # Deployment sudo grants root only; root then drops to the fixed
        # service identity. A direct sudo -u from feam-deploy is denied.
        return self.root(['/usr/sbin/runuser', '-u', 'feam-' + name, '--', *argv], timeout)

    def service(self, name, args, timeout=60):
        require(name in SERVICES, 'unknown fixed service')
        release = self.config['expected']['release_sha256']
        return self.as_service(name, ['/opt/feam/services/' + release + '/' + name,
                                      *args, '-config', '/etc/feam/services/' + name + '/config.json'], timeout)

    def broker(self, path, post=False):
        require(path in ('/v1/status', '/v1/run/drain'), 'fixed broker operation required')
        args = ['/usr/bin/curl', '--silent', '--show-error', '--fail',
                '--max-time', '35', '--unix-socket', '/run/feam/services/broker/control.sock']
        if post:
            args += ['-H', 'Content-Type: application/json', '--data-binary', '{}']
        return self.as_service('controller', [*args, 'http://unix' + path], 40)

    def budget(self, action, *args):
        require(action in ('status', 'unknown', 'reconcile'), 'lifecycle cannot allocate or raise ceilings')
        cfg = self.config['budget']
        return self.command([cfg['binary'], '-action', action, '-ledger', cfg['ledger'],
                             '-encryption-key', cfg['encryption_key'], *args])


class Lifecycle:
    def __init__(self, config, save, transport):
        self.config = validate(config)
        self.state = config['state']
        self.save = save
        self.transport = transport
        self.resuming = False

    def persist(self):
        self.state['updated_at'] = timestamp()
        self.save(self.config)

    def step(self, name, action):
        previous = self.state['steps'].get(name)
        if previous and previous['state'] == 'completed':
            return previous.get('result')
        require(not previous or self.resuming, 'uncertain step requires its explicit operation acknowledgment')
        self.state['steps'][name] = {'state': 'started', 'started_at': timestamp()}
        self.persist()  # intent is durable before the potentially mutating call
        result = action()
        self.state['steps'][name] = {'state': 'completed', 'completed_at': timestamp(), 'result': result}
        self.persist()
        return result

    def begin(self, action, resume=None):
        if resume:
            require(action == 'stop' and self.state.get('operation') == 'stop' and
                    resume == self.state.get('operation_id') and self.state['phase'] in ('stopping', 'needs_review'),
                    'only the exact interrupted stop can resume')
            self.resuming = True
        else:
            require(self.state.get('operation') != 'stop' or self.state['phase'] not in ('stopping', 'needs_review'),
                    'interrupted stop requires explicit operation acknowledgment')
            self.state.update(operation=action, operation_id=str(uuid.uuid4()), steps={}, receipts={})
        self.state.update(desired='running' if action == 'start' else 'stopped', phase='starting' if action == 'start' else 'stopping')
        self.persist()

    def status(self):
        value = json.loads(self.transport.service('controller', ['-action', 'status']))
        require(value['protocol'] == 'feam.lifecycle-status.v1' and value['deployment_id'] == self.config['deployment_id'] and
                value['activation_generation'] == self.config['activation_generation'] and
                set(value['workspaces']) == set(self.config['workspace_ids']), 'remote deployment scope changed')
        return value

    def project(self):
        value = json.loads(self.transport.budget('status'))
        require(value['project_id'] == self.config['project_id'], 'wrong durable project ledger')
        self.project_snapshot = value
        run = value['runs'][self.config['expected']['allocation_id']]
        document = run['document']
        allocation = document['allocation']
        # project-budget emits Go's fixed struct key order; these values are
        # restricted ASCII IDs/model/timestamps and a base64 signature.
        encoded = json.dumps(document, separators=(',', ':'), ensure_ascii=False).encode()
        require(hashlib.sha256(encoded).hexdigest() == self.config['expected']['allocation_sha256'] and
                allocation['deployment_id'] == self.config['deployment_id'] and allocation['run_id'] == self.config['run_id'] and
                allocation['activation_generation'] == self.config['activation_generation'], 'allocation binding changed')
        return run

    def _archive_ledger(self):
        value = json.loads(self.transport.service('collector', ['-mode', 'export-archive-ledger'], 70))
        require(value['protocol'] == 'feam.archive-ledger.v1' and HEX.fullmatch(value['sha256']) and
                all(isinstance(value[key], list) for key in ('objects', 'deletions', 'splits')) and len(canonical(value)) <= LIMIT,
                'bounded fresh archive carryover ledger required')
        previous = self.state.get('archive_ledger')
        if previous:
            indexed = {obj['object_key']: obj for obj in value['objects']}
            for obj in previous['objects']:
                current = indexed.get(obj['object_key'])
                require(current is not None and all(current[key] == obj[key] for key in ('kind', 'participant_id', 'sha256')),
                        'prior archive object missing after recreation; import reviewed ledger first')
            deleted = {item['participant_id'] for item in value['deletions']}
            require(all(item['participant_id'] in deleted for item in previous['deletions']), 'withdrawal lineage was reset')
            require(all(item in value['splits'] for item in previous['splits']), 'held-out split lineage was reset')
        return value

    def preflight(self):
        for path, field in [('/etc/feam/services/runtime.json', 'runtime_manifest_sha256'),
                            ('/etc/feam/services/collector/policy.json', 'policy_sha256')]:
            actual = self.transport.root(['/usr/bin/sha256sum', path]).decode().split()[0]
            require(actual == self.config['expected'][field], 'remote reviewed configuration changed')
        for service in SERVICES:
            self.transport.root(['/usr/local/sbin/feam-services-runtime', 'verify', '--service', service])
        return self.status()

    def start(self):
        require(self.state['phase'] == 'ready' and self.state['desired'] == 'stopped', 'start requires a prepared fresh activation')
        run = self.project()
        allocation = run['document']['allocation']
        require(run['state'] == 'active' and datetime.fromisoformat(allocation['expires_at'].replace('Z', '+00:00')) > datetime.now(timezone.utc),
                'existing finite active allocation required; lifecycle never allocates')
        status = self.preflight()
        if len(self.project_snapshot['runs']) > 1:
            require(self.state.get('archive_ledger') is not None, 'prior runs require their durable archive baseline')
        if self.state.get('archive_ledger') is not None:
            self._archive_ledger()  # fails if a fresh collector omitted old archive/cap/withdrawal metadata
        for prior in self.project_snapshot['runs'].values():
            allocation_before = prior['document']['allocation']
            if prior['state'] != 'closed' and allocation_before['deployment_id'] != self.config['deployment_id']:
                evidence = self.state.get('prior_revocations', {}).get(allocation_before['deployment_id'], {})
                require(evidence.get('protocol') == 'feam.external-revocation.v1' and
                        evidence.get('deployment_id') == allocation_before['deployment_id'] and
                        evidence.get('activation_generation') == allocation_before['activation_generation'] and
                        HEX.fullmatch(evidence.get('evidence_sha256', '')) and
                        all(evidence.get(key) is True for key in ('route_revoked', 'membership_writer_revoked', 'model_credential_revoked')),
                        'unclosed prior deployment requires reviewed external revocation evidence')
        require(status['desired'] == 'stopped' and not status['pending_jobs'] and not status['pending_snapshots'] and
                not status['pending_reclaims'], 'deployment must be settled and stopped before activation')
        self.begin('start')
        try:
            self.step('enable_services', lambda: self._systemd(['enable', *UNITS]))
            for service in SERVICES:
                self.step('start_' + service, lambda service=service: self._systemd(['start', 'feam-' + service + '.service']))
            broker = json.loads(self.transport.broker('/v1/status'))
            require(not broker['usage']['paused'] and broker['recording'] and
                    broker['usage']['allocation_usd_micros'] == allocation['amount_usd_micros'], 'private broker readiness failed')
            self.step('activate', lambda: self._controller('activate'))
            require(self.status()['desired'] == 'running', 'activation outcome not verified')
            self.step('health_timer', lambda: self._systemd(['enable', '--now', 'feam-service-health.timer']))
            self.state['phase'] = 'running'
            self.persist()
        except Exception:
            self.state['phase'] = 'needs_review'
            self.persist()
            raise

    def _systemd(self, args):
        self.transport.root(['/usr/bin/systemctl', *args], 75)

    def _controller(self, action):
        self.transport.service('controller', ['-action', action])

    def _drain(self):
        receipt = json.loads(self.transport.broker('/v1/run/drain', post=True))
        self.validate_drain(receipt)
        return receipt

    def validate_drain(self, receipt):
        require(receipt['protocol'] == 'feam.broker-drain.v1' and receipt['provenance'] == 'broker_observed' and
                receipt['allocation_id'] == self.config['expected']['allocation_id'] and
                receipt['allocation_sha256'] == self.config['expected']['allocation_sha256'] and
                receipt['deployment_id'] == self.config['deployment_id'] and
                receipt['activation_generation'] == self.config['activation_generation'] and
                receipt['usage']['paused'] and receipt['usage']['reserved_usd_micros'] == 0,
                'drain receipt does not prove this stopped allocation')

    def _wait_stopped(self):
        deadline = time.monotonic() + 120
        while True:
            value = self.status()
            require(value['desired'] == 'stopped', 'remote stop intent changed')
            if not value['pending_jobs'] and not value['pending_snapshots'] and not value['pending_reclaims'] and \
                    all(state in ('stopped', 'deleted') for state in value['workspaces'].values()):
                return value
            require(time.monotonic() < deadline, 'workspace stop incomplete; preserve runtime and unknown jobs')
            time.sleep(1)

    def _containers_stopped(self):
        raw = self.transport.as_service('controller', ['/usr/bin/curl', '--silent', '--fail',
                                     '--max-time', '10', '--unix-socket', '/run/user/2101/feam-docker.sock',
                                     'http://docker/v1.47/containers/json?all=true'])
        containers = json.loads(raw)
        require(isinstance(containers, list) and len(containers) <= 128, 'runtime inventory incomplete')
        owned = [item for item in containers if item.get('Labels', {}).get('feam.deployment') == self.config['deployment_id']]
        require(all(item['State'] in ('exited', 'created', 'dead') for item in owned), 'deployment still has active containers')
        return {'owned_containers': len(owned), 'running': 0}

    def _retain_allocation(self):
        run = self.project()
        if run['state'] == 'active':
            self.transport.budget('unknown', '-actor', self.config['actor'], '-allocation-id', self.config['expected']['allocation_id'])
        require(self.project()['state'] in ('unknown', 'closed'), 'project reservation not durably retained')

    def _archive(self):
        proof = json.loads(self.transport.service('collector', ['-mode', 'archive-verify', '-include-inventory'], 70))
        inventory = proof.get('inventory', [])
        require(proof['protocol'] == 'feam.archive-verification.v1' and proof['status'] == 'verified' and
                HEX.fullmatch(proof['sha256']) and isinstance(inventory, list), 'archive proof missing retained inventory')
        inventory_hash = hashlib.sha256()
        require(len(inventory) == sum(proof[name] for name in ('batches', 'exports', 'deletions', 'absent')),
                'archive inventory is incomplete')
        for item in inventory:
            require(set(item) == {'kind', 'object_key', 'sha256', 'state'} and item['kind'] in ('batch', 'export', 'deletion') and
                    item['state'] in ('verified', 'deleted') and HEX.fullmatch(item['sha256']) and
                    re.fullmatch(r'[A-Za-z0-9_./-]+', item['object_key']) and '..' not in item['object_key'].split('/'),
                    'archive object proof invalid')
            ordered = {key: item[key] for key in ('kind', 'object_key', 'sha256', 'state')}
            inventory_hash.update(json.dumps(ordered, separators=(',', ':')).encode() + b'\n')
        require(inventory_hash.hexdigest() == proof['sha256'], 'archive inventory hash mismatch')
        proof['inventory'] = inventory
        # Preserve the private inventory in encrypted operator state before any
        # preparation receipt. No event bodies or identity mapping are copied.
        require(len(canonical(proof)) <= LIMIT, 'archive inventory requires a separate bounded export before teardown')
        return proof

    def stop(self, resume=None):
        require(self.state['phase'] not in ('stopped_verified', 'teardown_prepared'), 'site already stopped; use prepare-teardown or status')
        self.begin('stop', resume)
        try:
            self.project()  # missing/corrupt durable state is never silently recreated
            self.preflight()
            self.step('stop_intent', lambda: self._controller('stop-intent'))
            require(self.status()['desired'] == 'stopped', 'stop intent not persisted')
            self.step('disable_health', lambda: self._systemd(['disable', '--now', 'feam-service-health.timer']))
            drain = self.step('drain', self._drain)
            self.state['receipts']['broker'] = drain
            self.persist()
            self.step('retain_allocation', self._retain_allocation)
            self.step('workspaces_stopped', self._wait_stopped)
            self._containers_stopped()
            self.step('stop_writers', lambda: self._systemd(['disable', '--now', 'feam-gateway.service', 'feam-pipeline.service',
                                                           'feam-reconciler.service', 'feam-controller.service', 'feam-broker.service']))
            self.step('stop_collector', lambda: self._systemd(['disable', '--now', 'feam-collector.service']))
            self.step('flush_archive', lambda: self.transport.service('collector', ['-mode', 'flush'], 70).decode())
            self.state['receipts']['archive'] = self._archive()
            self.state['archive_ledger'] = self._archive_ledger()
            self.state['phase'] = 'stopped_verified'
            self.persist()
        except Exception:
            self.state['phase'] = 'needs_review'
            self.persist()
            raise

    def stopped_proof(self):
        self._wait_stopped()
        runtime = self._containers_stopped()
        for unit in [*UNITS, 'feam-service-health.timer']:
            raw = self.transport.root(['/usr/bin/systemctl', 'show', unit, '--property=ActiveState,UnitFileState']).decode()
            fields = dict(line.split('=', 1) for line in raw.splitlines() if '=' in line)
            require(fields.get('ActiveState') == 'inactive' and fields.get('UnitFileState') == 'disabled',
                    'owned services must remain inactive and disabled')
        return runtime

    def prepare_teardown(self, revocations):
        require(self.state['phase'] in ('stopped_verified', 'teardown_prepared') and self.state['desired'] == 'stopped',
                'verified stop required before teardown preparation')
        self.state['phase'] = 'stopped_verified'
        self.state['receipts'].pop('teardown', None)
        self.persist()  # an old preparation cannot survive a failed fresh check
        require(set(revocations) == {'protocol', 'deployment_id', 'activation_generation', 'site_id', 'route_revoked',
                                     'membership_writer_revoked', 'model_credential_revoked', 'evidence_sha256'} and
                revocations['protocol'] == 'feam.external-revocation.v1' and
                revocations['deployment_id'] == self.config['deployment_id'] and
                revocations['activation_generation'] == self.config['activation_generation'] and
                revocations['site_id'] == self.config['site_id'] and HEX.fullmatch(revocations['evidence_sha256']) and
                all(revocations[k] is True for k in ('route_revoked', 'membership_writer_revoked', 'model_credential_revoked')),
                'reviewed external route and credential revocation evidence required')
        self.preflight()
        runtime = self.stopped_proof()
        self.validate_drain(self.state['receipts']['broker'])
        archive = self._archive()  # fresh independent readback, never an old watermark alone
        self.state['archive_ledger'] = self._archive_ledger()
        copy_record = self.state.get('mac_archive_copy')
        require(isinstance(copy_record, dict), 'independently verified Mac archive copy required')
        copy_receipt = archive_copy.verify_copy(copy_record['directory'], archive, self.state['archive_ledger'])
        require(copy_receipt == copy_record['receipt'] and
                copy_receipt['deployment_id'] == self.config['deployment_id'],
                'Mac archive copy receipt changed')
        run = self.project()
        require(run['state'] in ('unknown', 'closed'), 'allocation must remain reserved or be independently reconciled')
        self.state['receipts']['archive'] = archive
        self.state['receipts']['revocations'] = revocations
        self.state['receipts']['teardown'] = {'protocol': 'feam.teardown-preparation.v1', 'deployment_id': self.config['deployment_id'],
            'activation_generation': self.config['activation_generation'], 'site_id': self.config['site_id'],
            'broker_receipt_sha256': digest(self.state['receipts']['broker']), 'archive_receipt_sha256': digest(archive),
            'mac_copy_receipt_sha256': digest(copy_receipt),
            'revocation_receipt_sha256': digest(revocations), 'budget_state': run['state'], 'runtime': runtime,
            'prepared_at': timestamp(), 'destruction_authorized': False}
        self.state['phase'] = 'teardown_prepared'
        self.persist()

    def copy_archive(self, output_directory):
        require(self.state['phase'] in ('stopped_verified', 'teardown_prepared') and self.state['desired'] == 'stopped',
                'verified stop required before copying the archive')
        self.state['phase'] = 'stopped_verified'
        self.state['receipts'].pop('teardown', None)
        self.persist()  # an interrupted replacement cannot retain an older preparation
        self.preflight()
        self.stopped_proof()
        raw_config = self.transport.as_service('collector', ['/usr/bin/cat',
                                            '/etc/feam/services/collector/config.json'])
        collector_config = json.loads(raw_config)
        require(collector_config.get('archive_mode') == 'local' and
                collector_config.get('archive_directory') == archive_copy.REMOTE_ARCHIVE,
                'fixed local collector archive required for Mac copy')
        proof = self._archive()
        ledger = self._archive_ledger()
        receipt = archive_copy.remote_copy(self.config['ssh'], output_directory, proof, ledger,
                                           self.config['deployment_id'])
        old_copies = list(self.state.get('mac_archive_copies', []))
        if not old_copies and self.state.get('mac_archive_copy'):
            old_copies = [self.state['mac_archive_copy']]
        require(all(item['directory'] != str(Path(output_directory)) for item in old_copies),
                'new copy directory required')
        for old_copy in old_copies:
            archive_copy.maintain_copy(old_copy['directory'], proof, ledger)
        current_copy = {'directory': str(Path(output_directory)), 'receipt': receipt}
        self.state['mac_archive_copies'] = [*old_copies, current_copy]
        self.state['mac_archive_copy'] = current_copy
        self.state['archive_ledger'] = ledger
        self.state['receipts']['archive'] = proof
        self.state['receipts'].pop('teardown', None)
        self.state['phase'] = 'stopped_verified'
        self.persist()
        return receipt

    def prepare_next_run(self, document, service_vars, output_directory, operator_configuration):
        """Produce reviewable rotation inputs; never apply them or issue funds."""
        require(self.state['phase'] in ('stopped_verified', 'teardown_prepared') and self.state['desired'] == 'stopped',
                'verified stop required for next-run preparation')
        self.preflight()
        self.stopped_proof()
        baseline = self._archive_ledger()
        self.project()
        allocation = document['allocation']
        new_id = allocation['allocation_id']
        require(new_id != self.config['expected']['allocation_id'] and allocation['run_id'] != self.config['run_id'] and
                allocation['project_id'] == self.config['project_id'] and allocation['deployment_id'] == self.config['deployment_id'] and
                allocation['activation_generation'] == self.config['activation_generation'], 'next run must be a new reserved ID on this same stopped site')
        issued = self.project_snapshot['runs'][new_id]
        require(issued['state'] == 'active' and issued['document'] == document and
                datetime.fromisoformat(allocation['expires_at'].replace('Z', '+00:00')) > datetime.now(timezone.utc),
                'new signed document must already be reserved in the same durable project ledger')
        old_raw = self.transport.root(['/usr/bin/cat', '/etc/feam/services/runtime.json'])
        require(hashlib.sha256(old_raw).hexdigest() == self.config['expected']['runtime_manifest_sha256'], 'runtime manifest changed during review')
        manifest = json.loads(old_raw)
        raw = self.transport.as_service('broker', ['/usr/bin/cat', '/etc/feam/services/broker/config.json'])
        require(hashlib.sha256(raw).hexdigest() == manifest['config_sha256']['broker'], 'broker configuration changed during review')
        broker = json.loads(raw)
        root = '/home/feam-service-data/operations/broker/'
        for field in ('ledger', 'capabilities', 'receipt_file'):
            require(str(Path(broker[field]).parent) == root.rstrip('/'), 'unexpected broker state parent')
        broker.update(ledger=root + 'run-' + allocation['run_id'] + '.sqlite',
                      capabilities=root + 'capabilities-' + allocation['run_id'] + '.sqlite',
                      receipt_file=root + 'drain-' + allocation['run_id'] + '.json')
        require(broker['allocation'] == '/etc/feam/services/broker/allocation.json', 'fixed signed allocation destination required')
        require(service_vars['feam_services_release_sha256'] == manifest['release_sha256'] and
                service_vars['feam_workspace_ids'] == manifest['workspace_ids'] and
                service_vars['feam_service_mount_uuids'] == manifest['mounts'] and
                set(service_vars['feam_service_configs']) == set(SERVICES), 'service variables broaden the reviewed scope')
        for name in SERVICES:
            require(service_vars['feam_service_configs'][name]['sha256'] == manifest['config_sha256'][name],
                    'service variables do not describe the current reviewed configuration')
        directory = Path(output_directory)
        vault.private_directory(directory)
        broker_bytes = json.dumps(broker, indent=2, sort_keys=True).encode() + b'\n'
        manifest['config_sha256']['broker'] = hashlib.sha256(broker_bytes).hexdigest()
        # This matches Ansible's to_nice_json default used by the services role.
        manifest_bytes = json.dumps(manifest, indent=4, sort_keys=True).encode()
        allocation_bytes = json.dumps(document, indent=2).encode() + b'\n'
        variables = copy.deepcopy(service_vars)
        variables['feam_service_configs']['broker'] = {'source': str(directory / 'broker.json'), 'sha256': hashlib.sha256(broker_bytes).hexdigest()}
        variables['feam_services_private_files'] = [entry for entry in variables.get('feam_services_private_files', [])
                                                  if (entry['service'], entry['name']) != ('broker', 'allocation.json')]
        variables['feam_services_private_files'].append({'service': 'broker', 'name': 'allocation.json', 'source': str(directory / 'allocation.json')})
        variables.update(feam_services_initialize=['broker'], feam_services_migrate=[], feam_services_start=False, feam_services_enable=False)
        candidate = copy.deepcopy(operator_configuration)
        next_config = copy.deepcopy(self.config)
        next_config['run_id'] = allocation['run_id']
        ordered_document = issued['document']
        next_config['expected'].update(allocation_id=new_id,
            allocation_sha256=hashlib.sha256(json.dumps(ordered_document, separators=(',', ':'), ensure_ascii=False).encode()).hexdigest(),
            runtime_manifest_sha256=hashlib.sha256(manifest_bytes).hexdigest())
        next_config['state'] = {'desired': 'stopped', 'phase': 'ready', 'archive_ledger': baseline,
            'previous_run': {'run_id': self.config['run_id'], 'allocation_id': self.config['expected']['allocation_id'],
                             'broker_receipt_sha256': digest(self.state['receipts']['broker'])},
            'mac_archive_copy': copy.deepcopy(self.state.get('mac_archive_copy')),
            'mac_archive_copies': copy.deepcopy(self.state.get('mac_archive_copies', [])),
            'prior_revocations': copy.deepcopy(self.state.get('prior_revocations', {}))}
        candidate['site_lifecycle'] = next_config
        outputs = {'broker.json': broker_bytes, 'allocation.json': allocation_bytes, 'runtime-manifest.json': manifest_bytes,
                   'service-vars.json': json.dumps(variables, indent=2).encode() + b'\n',
                   'operator-next.json': json.dumps(candidate, indent=2).encode() + b'\n'}
        require(not any((directory / name).exists() or (directory / name).is_symlink() for name in outputs), 'rotation outputs already exist; inspect them rather than overwrite')
        for name, contents in outputs.items():
            vault.new_private(directory / name, contents)
        return {'protocol': 'feam.next-run-preparation.v1', 'run_id': allocation['run_id'],
                'files_sha256': {name: hashlib.sha256(contents).hexdigest() for name, contents in outputs.items()},
                'allocation_issued': False, 'host_changed': False, 'vault_changed': False}

    def reconcile(self, evidence):
        require(self.state['phase'] in ('stopped_verified', 'teardown_prepared'), 'verified stop required for budget closure')
        require(set(evidence) == {'protocol', 'allocation_id', 'allocation_sha256', 'broker_receipt_sha256',
                                  'verified_cost_usd_micros', 'provider_evidence_sha256'} and
                evidence['protocol'] == 'feam.provider-cost-review.v1' and
                evidence['allocation_id'] == self.config['expected']['allocation_id'] and
                evidence['allocation_sha256'] == self.config['expected']['allocation_sha256'] and
                evidence['broker_receipt_sha256'] == digest(self.state['receipts']['broker']) and
                type(evidence['verified_cost_usd_micros']) is int and evidence['verified_cost_usd_micros'] >= 0 and
                HEX.fullmatch(evidence['provider_evidence_sha256']), 'independently reviewed provider cost evidence required')
        run = self.project()
        require(self.state['receipts']['broker']['usage']['settled_usd_micros'] <= evidence['verified_cost_usd_micros'] <=
                run['document']['allocation']['amount_usd_micros'], 'reviewed cost contradicts known charges or issued allocation')
        expected = digest(evidence)
        if run['state'] == 'closed':
            require(run['reconciliation_sha256'] == expected and run['settled_usd_micros'] == evidence['verified_cost_usd_micros'],
                    'closed ledger has different verified evidence; no overwrite')
        # Retain the exact evidence before the idempotent project-ledger action.
        self.state['receipts']['provider_review'] = evidence
        self.persist()
        if run['state'] != 'closed':
            self.transport.budget('reconcile', '-actor', self.config['actor'], '-allocation-id', evidence['allocation_id'],
                                  '-amount', str(evidence['verified_cost_usd_micros']), '-evidence-sha256', expected)
        run = self.project()
        require(run['state'] == 'closed' and run['reconciliation_sha256'] == expected and
                run['settled_usd_micros'] == evidence['verified_cost_usd_micros'], 'ledger reconciliation conflict')
        self.state['receipts']['budget_reconciliation'] = {'sha256': expected, 'verified_cost_usd_micros': run['settled_usd_micros']}
        self.persist()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['status', 'start', 'stop', 'copy-archive', 'prepare-teardown', 'reconcile', 'prepare-next-run'])
    parser.add_argument('--vault', type=Path, required=True)
    parser.add_argument('--key', type=Path, required=True)
    parser.add_argument('--expected-revision', type=int, required=True)
    parser.add_argument('--resume-operation')
    parser.add_argument('--evidence', type=Path)
    parser.add_argument('--service-vars', type=Path)
    parser.add_argument('--output-directory', type=Path)
    args = parser.parse_args()
    try:
        vault.private_directory(args.vault.parent)
        fd = os.open(str(args.vault) + '.lifecycle.lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, 'r+b') as lock:
            info = os.fstat(lock.fileno())
            require(stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid() and not info.st_mode & 0o077 and info.st_nlink == 1,
                    'private single-writer lifecycle lock required')
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            record = vault.decode(args.vault, args.key)
            require(record['revision'] == args.expected_revision, 'stale operator record; inspect current revision')
            configuration = record['configuration']
            config = validate(configuration['site_lifecycle'])
            revision = record['revision']

            def save(updated):
                nonlocal revision
                configuration['site_lifecycle'] = copy.deepcopy(updated)
                with tempfile.NamedTemporaryFile(dir=args.vault.parent, prefix='.lifecycle-', delete=False) as output:
                    path = Path(output.name)
                    output.write(canonical(configuration)); output.flush(); os.fsync(output.fileno())
                try:
                    revision = vault.seal(path, args.vault, args.key, revision)['revision']
                finally:
                    path.unlink(missing_ok=True)

            if args.action != 'status':
                engine = Lifecycle(config, save, Transport(config))
                if args.action == 'start':
                    require(args.resume_operation is None, 'unknown start is not automatically replayed; stop or review it')
                    engine.start()
                elif args.action == 'stop':
                    engine.stop(args.resume_operation)
                elif args.action == 'copy-archive':
                    require(args.output_directory is not None and args.resume_operation is None,
                            'new private Mac archive directory required')
                    engine.copy_archive(args.output_directory)
                elif args.action == 'prepare-next-run':
                    require(args.evidence is not None and args.service_vars is not None and args.output_directory is not None and
                            args.resume_operation is None, 'new signed allocation, current service variables and private output directory required')
                    import yaml
                    proof = engine.prepare_next_run(json.loads(vault.private_read(args.evidence)),
                        yaml.safe_load(vault.private_read(args.service_vars)), args.output_directory, configuration)
                    print(json.dumps(proof))
                    return
                else:
                    require(args.evidence is not None and args.resume_operation is None, 'private reviewed evidence file required')
                    evidence = json.loads(vault.private_read(args.evidence))
                    if args.action == 'prepare-teardown':
                        engine.prepare_teardown(evidence)
                    else:
                        engine.reconcile(evidence)
            state = configuration['site_lifecycle']['state']
            print(json.dumps({'revision': revision, 'desired': state['desired'], 'phase': state['phase'],
                              'operation_id': state.get('operation_id'),
                              'teardown_prepared': state['phase'] == 'teardown_prepared', 'destruction_authorized': False}))
    except Exception:
        raise SystemExit('Site lifecycle incomplete. No compute deletion or new allocation was attempted. Inspect encrypted state and private service evidence before resuming.') from None


if __name__ == '__main__':
    main()
