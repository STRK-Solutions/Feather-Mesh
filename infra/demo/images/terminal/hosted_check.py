#!/usr/bin/env python3
"""Fail visibly before participant launch when fixed hosted inputs are absent."""
from pathlib import Path
import os
import stat
import tomllib
import json
import ssl
import sys
import urllib.request


def validate(config_root=Path('/run/feam-config'), socket=Path('/run/feam-model/broker.sock')):
    for name in ('ca.pem', 'cert.pem', 'key.pem', 'capability', 'feam/agent.toml'):
        path = config_root / name
        info = path.lstat()
        if not stat.S_ISREG(info.st_mode) or info.st_size == 0 or not os.access(path, os.R_OK):
            raise ValueError('required hosted file is not a readable regular file')
        # A named ACL may expose read-only access to this container's mapped UID
        # through the group-class mask. Deployment verifies group::--- and only
        # that exact named reader; mode bits alone cannot prove the ACL mapping.
        if name in ('key.pem', 'capability') and stat.S_IMODE(info.st_mode) & 0o027:
            raise ValueError('workspace credential permissions are too broad')
    if not stat.S_ISSOCK(socket.lstat().st_mode):
        raise ValueError('private model socket unavailable')
    profile = tomllib.loads((config_root / 'feam/agent.toml').read_text())
    if profile.get('schema_version') != 1 or profile.get('default_profile') != 'phase1-demo':
        raise ValueError('fixed participant profile unavailable')
    selected = profile['profiles']['phase1-demo']
    expected = {
        'backend': 'router', 'base_url': 'https://127.0.0.1:8443/v1',
        'loopback_ca_file': '/run/feam-config/ca.pem',
        'model': 'deepseek/deepseek-v4.1-flash', 'api_key_env': 'FEAM_BROKER_CAPABILITY',
        'allowed_providers': ['deepinfra/fp8'], 'allow_provider_fallbacks': False,
        'reasoning_enabled': False,
    }
    if any(selected.get(key) != value for key, value in expected.items()):
        raise ValueError('participant profile differs from fixed broker policy')
    if selected.get('context_policy') not in ('metadata-only', 'synthetic-demo'):
        raise ValueError('participant disclosure profile unavailable')
    # Controller stages a non-authorizing placeholder before the gateway has
    # committed the new workspace generation. Actual requests still require the
    # fresh 43-character capability enforced by the adapter and broker.
    capability = (config_root / 'capability').read_text().strip()
    if not capability or len(capability) > 128:
        raise ValueError('workspace capability unavailable')


def status():
    context = ssl.create_default_context(cafile='/run/feam-config/ca.pem')
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}),
                                         urllib.request.HTTPSHandler(context=context))
    request = urllib.request.Request('https://127.0.0.1:8443/v1/usage',
                                     headers={'Authorization': 'Bearer workspace-adapter'})
    with client.open(request, timeout=10) as response:
        value = json.loads(response.read(65537))
    if value.get('recording') is not True:
        raise ValueError('recorded participant service unavailable')
    run = value['project_run']
    print('FEAM assisted workspace. Model: DeepSeek V4.1 Flash. Recording active.')
    print('Run budget: ${:.6f}; spent ${:.6f}; reserved/unknown ${:.6f}; queued {}.'.format(
        run['allocation_usd_micros'] / 1000000, run['settled_usd_micros'] / 1000000,
        (run['reserved_usd_micros'] + run['unknown_usd_micros']) / 1000000,
        value['queued']))
    if run['paused']:
        raise ValueError('model budget paused')


if __name__ == '__main__':
    try:
        validate()
        if sys.argv[1:] == ['--status']:
            status()
        elif sys.argv[1:]:
            raise ValueError('unexpected arguments')
    except (OSError, ValueError, KeyError, TypeError):
        raise SystemExit('Hosted FEAM configuration is incomplete; participant launch refused. Contact the operator.')
