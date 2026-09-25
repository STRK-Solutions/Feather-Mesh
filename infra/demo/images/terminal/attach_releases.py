#!/usr/bin/env python3
"""Attach only controller-mounted immutable releases to the practice consumer.

The controller owns mount authorization and digest verification. This fixed
startup helper owns only the marked peer-route section; manifests remain FEAM
authority and existing practice routes and publication history stay intact.
"""
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import tempfile
import tomllib

BEGIN = '# BEGIN FEAM controller-managed release peers\n'
END = '# END FEAM controller-managed release peers\n'
SLUG = re.compile(r'[a-z][a-z0-9-]{0,62}\Z')
DIGEST = re.compile(r'[0-9a-f]{64}\Z')


def unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('duplicate JSON key')
        result[key] = value
    return result


def ordinary(path, directory=False, max_bytes=262144):
    info = path.lstat()
    expected = stat.S_ISDIR if directory else stat.S_ISREG
    if not expected(info.st_mode) or info.st_mode & stat.S_IWOTH:
        raise ValueError('required ordinary non-world-writable file or directory')
    if not directory and info.st_size > max_bytes:
        raise ValueError('configuration exceeds byte limit')


def attach(project=Path('/workspace/demo/client with spaces'),
           config=Path('/run/feam-config/releases.json'),
           datasets=Path('/datasets')):
    ordinary(config)
    releases = json.loads(config.read_text(), object_pairs_hook=unique)
    if not isinstance(releases, list) or len(releases) > 32:
        raise ValueError('bounded release list required')
    managed, seen = [], set()
    for release in releases:
        if not isinstance(release, dict) or set(release) != {'bundle', 'digest', 'namespace', 'serving_root'}:
            raise ValueError('closed release identity required')
        bundle = release['bundle']
        if not isinstance(bundle, str) or not SLUG.fullmatch(bundle) or bundle in seen:
            raise ValueError('unique approved bundle required')
        if release['namespace'] != bundle or not isinstance(release['digest'], str) or not DIGEST.fullmatch(release['digest']):
            raise ValueError('invalid exact release identity')
        if release['serving_root'] != '/datasets/' + bundle + '/serving':
            raise ValueError('release must use its fixed dataset mount')
        # In production datasets is fixed /datasets. The argument permits only
        # local fixture tests; no browser/environment input controls main().
        serving = datasets / bundle / 'serving'
        for directory in (datasets, serving.parent, serving):
            ordinary(directory, directory=True)
        manifest = serving / 'manifest.json'
        ordinary(manifest, max_bytes=16 << 20)
        metadata = json.loads(manifest.read_text(), object_pairs_hook=unique)
        if metadata.get('namespace') != bundle or metadata.get('schema_version') != 1 or type(metadata.get('revision')) is not int or metadata['revision'] < 1:
            raise ValueError('manifest identity does not match approved release')
        seen.add(bundle)
        managed.append((bundle, release['digest'], release['serving_root']))
    for directory in (project, project / '.feam'):
        ordinary(directory, directory=True)
    path = project / '.feam/project.toml'
    ordinary(path)
    original = path.read_text()
    if original.count(BEGIN) != original.count(END) or original.count(BEGIN) > 1:
        raise ValueError('ambiguous managed peer section')
    base = original
    if BEGIN in original:
        start, end = original.index(BEGIN), original.index(END)
        if start >= end:
            raise ValueError('invalid managed peer section')
        base = original[:start] + original[end + len(END):]
    parsed = tomllib.loads(base)
    for peer in parsed.get('peers', []):
        if peer.get('alias', '').startswith('approved-') or peer.get('namespace') in seen:
            raise ValueError('practice peer conflicts with managed release')
    lines = [base.rstrip() + '\n\n', BEGIN]
    for bundle, digest, serving in sorted(managed):
        lines += ['# Release SHA-256: ' + digest + '\n', '[[peers]]\n',
                  'alias = ' + json.dumps('approved-' + bundle) + '\n',
                  'namespace = ' + json.dumps(bundle) + '\n',
                  'path = ' + json.dumps(serving) + '\n\n']
    lines.append(END)
    candidate = ''.join(lines)
    tomllib.loads(candidate)
    if candidate == original:
        return
    descriptor, temporary = tempfile.mkstemp(prefix='.project-grants-', dir=path.parent)
    try:
        with os.fdopen(descriptor, 'w') as output:
            output.write(candidate)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
        directory_fd = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory_fd)
        finally:
            os.close(directory_fd)
    finally:
        Path(temporary).unlink(missing_ok=True)


def main():
    attach()
    # Refresh validates the manifest through the shared FEAM parser. Revocation
    # removes the route before stale projections can be used for direct reads.
    subprocess.run(['/usr/local/bin/feam', '--project',
                    '/workspace/demo/client with spaces', '--format', 'json', 'refresh'],
                   stdin=subprocess.DEVNULL, check=True, timeout=30)


if __name__ == '__main__':
    main()
