#!/usr/bin/env python3
"""Render reviewed private U.04 service inputs; never applies them to a host."""
import argparse
import base64
import binascii
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys

import operator_vault as vault
import services_runtime as runtime

SERVICES = ('gateway', 'controller', 'broker', 'collector', 'pipeline', 'reconciler')
ARTIFACTS = (*SERVICES, 'feam')
SHA = re.compile(r'^[0-9a-f]{64}$')
UUID = re.compile(r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
ALLOCATION_FIELDS = ('protocol', 'allocation_id', 'project_id', 'run_id', 'deployment_id',
    'activation_generation', 'ledger_revision', 'amount_usd_micros',
    'request_limit_usd_micros', 'user_daily_limit_usd_micros', 'model', 'provider',
    'profile', 'input_price_usd_micros_per_million',
    'output_price_usd_micros_per_million', 'fee_basis_points', 'max_output_tokens',
    'not_before', 'expires_at')


def require(value, reason):
    if not value:
        raise ValueError(reason)


def private_bytes(path, limit=1 << 20):
    path = Path(path)
    vault.private_directory(path.parent)
    return vault.private_read(path, limit)


def output(directory, name, value):
    body = json.dumps(value, indent=2, sort_keys=True).encode() + b'\n'
    path = directory / name
    vault.new_private(path, body)
    return {'source': str(path), 'sha256': hashlib.sha256(body).hexdigest()}


def verify_live_signature(signed, public_key):
    from cryptography.exceptions import InvalidSignature
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey
    require(set(signed) == {'allocation', 'signature'} and
            set(signed['allocation']) == set(ALLOCATION_FIELDS),
            'complete signed live allocation required')
    try:
        signature = base64.b64decode(signed['signature'], validate=True)
        require(len(signature) == 64, 'Ed25519 signature required')
        payload = json.dumps({key: signed['allocation'][key] for key in ALLOCATION_FIELDS},
                             separators=(',', ':'), ensure_ascii=False).encode()
        Ed25519PublicKey.from_public_bytes(public_key).verify(signature, payload)
    except (InvalidSignature, ValueError, TypeError, binascii.Error):
        raise ValueError('real signed allocation verification failed') from None


def render(inputs, topology, destination):
    destination = Path(destination)
    require(destination.is_absolute() and not destination.exists() and not destination.is_symlink(),
            'fresh absolute output directory required')
    vault.private_directory(destination.parent)
    require(inputs['phase'] in ('inert_install', 'final') and inputs['release_sha256'] == topology['native_go_source_sha256'] and
            SHA.fullmatch(inputs['release_sha256']) and inputs['frontend_uid'] == 2109,
            'reviewed release, phase and separate edge identity required')
    require(set(inputs['artifact_sources']) == set(ARTIFACTS) and
            all(Path(v).is_absolute() for v in inputs['artifact_sources'].values()),
            'exact native artifact source paths required')
    hashes = {name: topology['native_go_artifact_sha256'][name] for name in SERVICES}
    hashes['feam'] = inputs['feam_sha256']
    require(all(SHA.fullmatch(hashes[name]) for name in ARTIFACTS), 'artifact digest missing')
    require(inputs['image_id'].startswith('sha256:') and SHA.fullmatch(inputs['image_id'][7:]) and
            SHA.fullmatch(inputs['dataset_sha256']) and SHA.fullmatch(inputs['profile_sha256']),
            'reviewed image, dataset and profile digests required')
    require(hashlib.sha256(private_bytes(inputs['profile_source'])).hexdigest() == inputs['profile_sha256'],
            'private profile hash changed')
    mode = inputs.get('broker_mode', 'fake')
    require(mode in ('fake', 'live') and (mode == 'fake' or inputs['phase'] == 'final'),
            'explicit final live or staged fake provider mode required')
    if mode == 'live':
        require(UUID.fullmatch(inputs['live_project_id']) and
                UUID.fullmatch(inputs['live_run_id']) and UUID.fullmatch(inputs['live_allocation_id']),
                'exact real project, run and allocation IDs required')
        previous_run = inputs['previous_fake_run_id']
        previous_allocation = inputs['previous_fake_allocation_id']
        require(UUID.fullmatch(previous_run) and UUID.fullmatch(previous_allocation) and
                inputs['live_run_id'] != previous_run and
                inputs['live_allocation_id'] != previous_allocation,
                'live run and allocation must differ from preserved synthetic identifiers')
        previous_document = private_bytes(inputs['previous_fake_allocation_source'])
        previous_signed = json.loads(previous_document)
        previous_public = private_bytes(inputs['previous_fake_public_key_source'], 32)
        require(previous_signed['allocation']['deployment_id'] == topology['deployment_id'] and
                previous_signed['allocation']['activation_generation'] == topology['activation_generation'] and
                previous_signed['allocation']['run_id'] == previous_run and
                previous_signed['allocation']['allocation_id'] == previous_allocation and
                len(previous_public) == 32,
                'preserved synthetic signed allocation binding required')
        allocation_source = inputs['live_allocation_source']
        public_key_source = inputs['live_public_key_source']
        run_id = inputs['live_run_id']
        allocation_id = inputs['live_allocation_id']
        upstream = private_bytes(inputs['upstream_key_source'], 4096)
        require(0 < len(upstream.strip()) <= 4096 and len(upstream) <= 4096,
                'bounded private live provider key required')
    else:
        allocation_source = inputs['fake_allocation_source']
        public_key_source = inputs['fake_public_key_source']
        run_id = inputs['fake_run_id']
        allocation_id = inputs['fake_allocation_id']
    signed = json.loads(private_bytes(allocation_source))
    public_key = private_bytes(public_key_source, 32)
    require(signed['allocation']['deployment_id'] == topology['deployment_id'] and
            signed['allocation']['activation_generation'] == topology['activation_generation'] and
            signed['allocation']['run_id'] == run_id and
            signed['allocation']['allocation_id'] == allocation_id and
            (mode == 'fake' or signed['allocation'].get('project_id') == inputs['live_project_id']) and
            len(public_key) == 32,
            'separate signed allocation binding required')
    if mode == 'live':
        verify_live_signature(signed, public_key)
    cf = inputs['cloudflare']
    require(re.fullmatch(r'https://[a-z0-9-]+\.cloudflareaccess\.com', cf['issuer']) and
            re.fullmatch(r'^[0-9a-f]{32}$', cf['account_id']) and
            cf['user_group'] != cf['admin_group'] and
            all(UUID.fullmatch(cf[k]) for k in ('user_group', 'admin_group')),
            'real closed Cloudflare issuer/account/groups required')
    workspace_ids = topology['workspace_ids']
    require(1 <= len(workspace_ids) <= 10 and len(set(workspace_ids)) == len(workspace_ids) and
            all(UUID.fullmatch(v) for v in workspace_ids), 'one to ten fixed unique workspaces required')
    hosts = ['feam.613202690.xyz', 'admin.613202690.xyz',
             *['u-' + wid + '.613202690.xyz' for wid in workspace_ids]]
    require(set(cf['audiences']) == set(hosts) and len(set(cf['audiences'].values())) == len(hosts) and
            all(SHA.fullmatch(v) for v in cf['audiences'].values()) and
            set(cf['group_names']) == {cf['user_group'], cf['admin_group']} and
            all(cf['group_names'].values()), 'exact distinct Access app audiences and group names required')
    reviewer_uid = topology['reviewer_uid']
    require(reviewer_uid == 1000 and reviewer_uid != inputs['frontend_uid'], 'separate owner-only reviewer required')
    owner_ids = inputs['owner_ids']
    require(len(owner_ids) == len(workspace_ids) and len(set(owner_ids)) == len(owner_ids) and
            all(UUID.fullmatch(v) for v in owner_ids), 'one fixed owner per workspace required')
    if inputs['phase'] == 'inert_install':
        require(owner_ids == inputs['preliminary_owner_ids'], 'inert install must use explicit provisional owners')
    else:
        require(not set(owner_ids) & set(inputs['preliminary_owner_ids']),
                'final install requires bootstrap account readback for every owner')
    participants = inputs['participant_ids']
    require(len(participants) == len(workspace_ids) and len(set(participants)) == len(participants) and
            all(UUID.fullmatch(v) for v in participants), 'one pseudonymous participant per workspace required')
    socket = lambda name: '/run/feam/services/' + name + '/control.sock'
    release = '/opt/feam/services/' + inputs['release_sha256']
    gateway = {'db': '/home/feam-service-data/operations/gateway/control.sqlite',
        'browser_socket': '/run/feam/services/gateway/browser.sock', 'control_socket': socket('gateway'),
        'issuer': cf['issuer'], 'read_uids': [2103, 2104, 2105, 2106, 2107],
        'controller_uid': 2103, 'reconciler_uid': 2107, 'pipeline_uid': 2106,
        'gateway': {'portal_host': hosts[0], 'admin_host': hosts[1], 'audiences': cf['audiences'],
                    'controller_socket': socket('controller'), 'pipeline_socket': socket('pipeline'),
                    'broker_socket': socket('broker'), 'model': 'deepseek/deepseek-v4.1-flash via deepinfra/fp8',
                    'recording': 'Structured research recording; 30-day retention',
                    'budget': 'Finite run allocation; usage from broker'}}
    endpoints = {}
    for wid, participant in zip(workspace_ids, participants):
        base = '/run/feam/workspaces/' + wid
        endpoints[wid] = {'terminal_directory': base + '/terminal', 'model_directory': base + '/model',
            'event_directory': base + '/events', 'config_directory': base + '/config',
            'participant_id': participant, 'software_sha256': hashes['feam'],
            'profile_sha256': inputs['profile_sha256'], 'dataset_sha256': inputs['dataset_sha256'],
            'synthetic': mode == 'fake'}
    controller = {'database': '/home/feam-service-data/operations/controller/lifecycle.sqlite',
        'socket': socket('controller'), 'gateway_socket': socket('gateway'),
        'runtime_socket': '/run/user/2101/feam-docker.sock', 'gateway_uid': 2102, 'broker_uid': 2104,
        'deployment_id': topology['deployment_id'], 'activation_generation': topology['activation_generation'],
        'image_id': inputs['image_id'], 'slots': topology['slots'],
        'workspaces': [{'id': wid, 'owner_id': owner, 'hostname': 'u-' + wid + '.613202690.xyz',
                        'deployment_id': topology['deployment_id'], 'generation': 1, 'state': 'stopped',
                        'slot_id': '', 'image_id': inputs['image_id'], 'assignment_version': 1,
                        'last_activity': '', 'warning_at': ''}
                       for wid, owner in zip(workspace_ids, owner_ids)],
        'broker_socket': socket('broker'), 'collector_socket': socket('collector'),
        'pipeline_socket': socket('pipeline'),
        'release_root': '/home/feam-service-data/datasets/pipeline/releases',
        'profile_path': '/etc/feam/services/controller/agent.toml', 'endpoints': endpoints}
    broker = {'ledger': '/home/feam-service-data/operations/broker/run-' + run_id + '.sqlite',
        'capabilities': '/home/feam-service-data/operations/broker/capabilities-' + run_id + '.sqlite',
        'allocation': '/etc/feam/services/broker/allocation.json',
        'operator_public_key': '/etc/feam/services/broker/operator.pub',
        'deployment_id': topology['deployment_id'], 'activation_generation': topology['activation_generation'],
        'authority_socket': socket('gateway'), 'control_socket': socket('broker'),
        'controller_socket': socket('controller'),
        'receipt_file': '/home/feam-service-data/operations/broker/drain-' + run_id + '.json',
        'controller_uid': 2103, 'gateway_uid': 2102,
        'upstream_key_file': '/etc/feam/services/broker/upstream.key' if mode == 'live' else '',
        'collector_socket': socket('collector'), 'software_sha256': hashes['feam'],
        'profile_sha256': inputs['profile_sha256'], 'dataset_sha256': inputs['dataset_sha256'],
        'sockets': [{'workspace_id': wid, 'path': '/run/feam/workspaces/' + wid + '/model/broker.sock'}
                    for wid in workspace_ids]}
    collector = {'db': '/home/feam-service-data/traces/collector/events.sqlite',
        'capabilities_db': '/home/feam-service-data/traces/collector/capabilities.sqlite',
        'control_socket': socket('collector'), 'gateway_socket': socket('gateway'),
        'workspace_sockets': {wid: '/run/feam/workspaces/' + wid + '/events/events.sock' for wid in workspace_ids},
        'broker_uid': 2104, 'controller_uid': 2103, 'verifier_uid': 2106,
        'reviewer_uids': [reviewer_uid], 'reviewer': inputs['reviewer'],
        'policy_file': '/etc/feam/services/collector/policy.json', 'archive_mode': 'local',
        'archive_directory': str(runtime.LOCAL_ARCHIVE), 'r2_credentials_file': '',
        'r2_account_id': '', 'r2_bucket': '', 'local_max_bytes': 20 << 30,
        'archive_max_bytes': 1 << 30}
    pipeline = {'root': '/home/feam-service-data/datasets/pipeline', 'feam': release + '/feam',
        'python': '/opt/feam/pipeline/bin/python', 'converter': '/opt/feam/importers/convert.py',
        'socket': socket('pipeline'), 'controller_uid': 2103, 'gateway_uid': 2102,
        'control_socket': socket('gateway')}
    reconciler = {'control_socket': socket('gateway'), 'account_id': cf['account_id'],
        'user_group': cf['user_group'], 'admin_group': cf['admin_group'],
        'group_names': cf['group_names'], 'token_file': '/etc/feam/services/reconciler/token'}
    configs = dict(gateway=gateway, controller=controller, broker=broker,
                   collector=collector, pipeline=pipeline, reconciler=reconciler)
    manifest = {'protocol': 'feam.services.v1', 'release_sha256': inputs['release_sha256'],
        'frontend_uid': inputs['frontend_uid'], 'mounts': topology['mount_uuids'],
        'workspace_ids': workspace_ids, 'config_sha256': {name: 'a' * 64 for name in SERVICES},
        'artifact_sha256': hashes, 'broker_mode': mode}
    for name, config in configs.items():
        runtime.validate_config(name, config, manifest)
    policy = [] if inputs['phase'] == 'inert_install' else [
        {'account_id': owner, 'participant_id': participant,
         'consent_reference': inputs['consent_reference'], 'eligible': True,
         'allow_text': False, 'synthetic': mode == 'fake'}
        for owner, participant in zip(owner_ids, participants)]
    destination.mkdir(mode=0o700)
    config_refs = {name: output(destination, name + '.json', value) for name, value in configs.items()}
    policy_ref = output(destination, 'collector-policy.json', policy)
    profile_ref = destination / 'controller-agent.toml'
    vault.new_private(profile_ref, private_bytes(inputs['profile_source']))
    allocation_ref = destination / 'broker-allocation.json'
    vault.new_private(allocation_ref, private_bytes(allocation_source))
    public_ref = destination / 'broker-operator.pub'
    vault.new_private(public_ref, public_key)
    token_ref = destination / 'reconciler-token'
    vault.new_private(token_ref, private_bytes(cf['token_source']))
    private_files = [
        {'service': 'controller', 'name': 'agent.toml', 'source': str(profile_ref)},
        {'service': 'collector', 'name': 'policy.json', 'source': policy_ref['source']},
        {'service': 'broker', 'name': 'allocation.json', 'source': str(allocation_ref)},
        {'service': 'broker', 'name': 'operator.pub', 'source': str(public_ref)},
        {'service': 'reconciler', 'name': 'token', 'source': str(token_ref)}]
    if mode == 'live':
        key_ref = destination / 'broker-upstream.key'
        vault.new_private(key_ref, upstream)
        private_files.append({'service': 'broker', 'name': 'upstream.key', 'source': str(key_ref)})
        vault.new_private(destination / 'previous-fake-allocation.json', previous_document)
        vault.new_private(destination / 'previous-fake-operator.pub', previous_public)
    vars_file = {'feam_services_release_sha256': inputs['release_sha256'],
        'feam_service_artifacts': {name: {'source': inputs['artifact_sources'][name], 'sha256': hashes[name]}
                                   for name in ARTIFACTS},
        'feam_service_configs': config_refs,
        'feam_service_mount_uuids': topology['mount_uuids'],
        'feam_services_controller_slots': {slot['path']: slot['filesystem_uuid'] for slot in topology['slots']},
        'feam_workspace_ids': workspace_ids, 'feam_gateway_frontend_uid': inputs['frontend_uid'],
        'feam_services_private_files': private_files, 'feam_services_broker_mode': mode,
        'feam_services_initialize': ['broker'] if mode == 'live' else [], 'feam_services_migrate': [],
        'feam_services_start': False, 'feam_services_enable': False}
    vars_ref = output(destination, 'service-vars.json', vars_file)
    repeat_vars_ref = None
    if mode == 'live':
        repeat_vars = dict(vars_file, feam_services_initialize=[])
        repeat_vars_ref = output(destination, 'service-vars-post-init.json', repeat_vars)
    receipt = {'protocol': 'feam.u04-service-render.v1', 'phase': inputs['phase'],
        'deployable': inputs['phase'] == 'final', 'services_start': False,
        'deployment_id': topology['deployment_id'], 'release_sha256': inputs['release_sha256'],
        'config_sha256': {name: ref['sha256'] for name, ref in config_refs.items()},
        'service_vars_sha256': vars_ref['sha256'], 'policy_sha256': policy_ref['sha256'],
        'broker_mode': mode, 'allocation_sha256': hashlib.sha256(allocation_ref.read_bytes()).hexdigest()}
    if mode == 'fake':
        receipt['fake_allocation_sha256'] = receipt['allocation_sha256']
    else:
        receipt['preserved_fake_allocation_sha256'] = hashlib.sha256(previous_document).hexdigest()
        receipt['preserved_fake_public_key_sha256'] = hashlib.sha256(previous_public).hexdigest()
        receipt['service_vars_post_init_sha256'] = repeat_vars_ref['sha256']
    output(destination, 'render-receipt.json', receipt)
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--inputs', type=Path, required=True)
    parser.add_argument('--topology', type=Path, required=True)
    parser.add_argument('--output-directory', type=Path, required=True)
    args = parser.parse_args()
    try:
        receipt = render(json.loads(private_bytes(args.inputs)), json.loads(private_bytes(args.topology)),
                         args.output_directory)
        print(json.dumps({'phase': receipt['phase'], 'deployable': receipt['deployable'],
                          'service_vars_sha256': receipt['service_vars_sha256']}))
    except Exception:
        raise SystemExit('Private U.04 service render failed; inspect reviewed inputs and retain existing outputs.') from None


if __name__ == '__main__':
    main()
