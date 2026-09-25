"""Local synthetic archive transfer and retained-copy checks; no SSH or host changes."""
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'scripts'))
import archive_copy

ID = '00000000-0000-4000-8000-000000000001'
BODY = b'{"redacted":"synthetic"}'
HASH = hashlib.sha256(BODY).hexdigest()
KEY = f'research/events/{ID}/{HASH}.json'
NAME = KEY.replace('/', '_')


def fixture():
    expiry = (datetime.now(timezone.utc) + timedelta(days=2)).isoformat()
    item = {'kind': 'batch', 'object_key': KEY, 'sha256': HASH, 'state': 'verified'}
    inventory_hash = hashlib.sha256(json.dumps(item, separators=(',', ':')).encode() + b'\n').hexdigest()
    proof = {'protocol': 'feam.archive-verification.v1', 'status': 'verified', 'inventory': [item],
             'sha256': inventory_hash, 'batches': 1, 'exports': 0, 'deletions': 0, 'absent': 0,
             'bytes': len(BODY), 'verified_at': datetime.now(timezone.utc).isoformat()}
    ledger = {'protocol': 'feam.archive-ledger.v1', 'sha256': 'a' * 64, 'objects': [{
        'id': ID, 'kind': 'batch', 'participant_id': ID, 'object_key': KEY, 'sha256': HASH,
        'state': 'verified', 'bytes': len(BODY), 'created_at': '', 'expires_at': expiry,
        'reviewer': ''}], 'deletions': [], 'splits': []}
    return proof, ledger


def tar_stream(entries):
    out = io.BytesIO()
    with tarfile.open(fileobj=out, mode='w') as archive:
        for name, data in entries:
            info = tarfile.TarInfo(name)
            info.size = len(data)
            archive.addfile(info, io.BytesIO(data))
    out.seek(0)
    return out


class ArchiveCopyTests(unittest.TestCase):
    def test_copy_verifies_content_and_private_metadata(self):
        proof, ledger = fixture()
        with tempfile.TemporaryDirectory() as base:
            parent = Path(base).resolve() / 'operator'
            parent.mkdir(mode=0o700)
            target = parent / 'copy'
            archive_copy.copy_stream(tar_stream([(NAME, BODY)]), target, proof, ledger, ID)
            self.assertEqual(archive_copy.verify_copy(target)['objects'], 1)
            self.assertEqual((target / NAME).stat().st_mode & 0o777, 0o600)
            (target / NAME).write_bytes(b'tampered')
            with self.assertRaises(ValueError):
                archive_copy.verify_copy(target)

    def test_rejects_symlink_and_extra_local_file(self):
        proof, ledger = fixture()
        with tempfile.TemporaryDirectory() as base:
            parent = Path(base).resolve() / 'operator'
            parent.mkdir(mode=0o700)
            target = parent / 'copy'
            archive_copy.copy_stream(tar_stream([(NAME, BODY)]), target, proof, ledger, ID)
            (target / 'extra').write_bytes(b'not inventoried')
            with self.assertRaises(ValueError):
                archive_copy.verify_copy(target)
            (target / 'extra').unlink()
            (target / NAME).unlink()
            os.symlink(parent / 'outside', target / NAME)
            with self.assertRaises(ValueError):
                archive_copy.verify_copy(target)

    def test_rejects_unexpected_and_missing_members(self):
        proof, ledger = fixture()
        for entries in ([], [(NAME, BODY), ('escape', BODY)], [(NAME, b'changed')]):
            with self.subTest(entries=entries), tempfile.TemporaryDirectory() as base:
                parent = Path(base).resolve() / 'operator'
                parent.mkdir(mode=0o700)
                with self.assertRaises(ValueError):
                    archive_copy.copy_stream(tar_stream(entries), parent / 'copy', proof, ledger, ID)

    def test_withdrawal_removes_old_retained_bytes(self):
        proof, ledger = fixture()
        with tempfile.TemporaryDirectory() as base:
            parent = Path(base).resolve() / 'operator'
            parent.mkdir(mode=0o700)
            target = parent / 'copy'
            archive_copy.copy_stream(tar_stream([(NAME, BODY)]), target, proof, ledger, ID)
            current_ledger = json.loads(json.dumps(ledger))
            current_ledger['objects'][0]['state'] = 'deleted'
            current_proof = json.loads(json.dumps(proof))
            current_proof['inventory'][0]['state'] = 'deleted'
            archive_copy.maintain_copy(target, current_proof, current_ledger)
            self.assertFalse((target / NAME).exists())
            archive_copy.verify_copy(target)

    def test_fresh_observation_timestamps_do_not_invalidate_identical_copy(self):
        proof, ledger = fixture()
        with tempfile.TemporaryDirectory() as base:
            parent = Path(base).resolve() / 'operator'
            parent.mkdir(mode=0o700)
            target = parent / 'copy'
            archive_copy.copy_stream(tar_stream([(NAME, BODY)]), target, proof, ledger, ID)
            fresh_proof = json.loads(json.dumps(proof))
            fresh_ledger = json.loads(json.dumps(ledger))
            fresh_proof['verified_at'] = '2026-09-26T00:00:00Z'
            fresh_ledger['exported_at'] = '2026-09-26T00:00:01Z'
            fresh_ledger['sha256'] = 'b' * 64
            archive_copy.verify_copy(target, fresh_proof, fresh_ledger)
            changed = json.loads(json.dumps(fresh_ledger))
            changed['splits'] = [{'dimension': 'held-out', 'value': 'different', 'split': 'train'}]
            with self.assertRaises(ValueError):
                archive_copy.verify_copy(target, fresh_proof, changed)
            changed = json.loads(json.dumps(fresh_ledger))
            changed['objects'][0]['sha256'] = 'c' * 64
            changed_proof = json.loads(json.dumps(fresh_proof))
            changed_proof['inventory'][0]['sha256'] = 'c' * 64
            with self.assertRaises(ValueError):
                archive_copy.verify_copy(target, changed_proof, changed)
            changed = json.loads(json.dumps(fresh_ledger))
            changed['deletions'] = [{'participant_id': ID, 'reason': 'withdrawal',
                                     'created_at': '2026-09-26T00:00:00Z', 'archive_state': 'verified'}]
            deletion_body = json.dumps({'created_at': changed['deletions'][0]['created_at'],
                                        'participant_id': ID, 'reason': 'withdrawal'}, separators=(',', ':')).encode()
            deletion_hash = hashlib.sha256(deletion_body).hexdigest()
            changed_proof = json.loads(json.dumps(fresh_proof))
            changed_proof['inventory'].append({'kind': 'deletion',
                'object_key': f'research/deletions/{ID}/{deletion_hash}.json',
                'sha256': deletion_hash, 'state': 'verified'})
            with self.assertRaises(ValueError):
                archive_copy.verify_copy(target, changed_proof, changed)

    def test_offline_expiry_removes_bytes_after_deadline(self):
        proof, ledger = fixture()
        deadline = datetime.now(timezone.utc) - timedelta(days=1)
        ledger['objects'][0]['expires_at'] = deadline.isoformat()
        with tempfile.TemporaryDirectory() as base:
            parent = Path(base).resolve() / 'operator'
            parent.mkdir(mode=0o700)
            target = parent / 'copy'
            archive_copy.copy_stream(tar_stream([(NAME, BODY)]), target, proof, ledger, ID)
            self.assertEqual(archive_copy.expire_copy(target), 1)
            self.assertFalse((target / NAME).exists())
            archive_copy.verify_copy(target)

    def test_interrupted_expiry_intent_resumes(self):
        proof, ledger = fixture()
        ledger['objects'][0]['expires_at'] = (datetime.now(timezone.utc) - timedelta(days=1)).isoformat()
        with tempfile.TemporaryDirectory() as base:
            parent = Path(base).resolve() / 'operator'
            parent.mkdir(mode=0o700)
            target = parent / 'copy'
            archive_copy.copy_stream(tar_stream([(NAME, BODY)]), target, proof, ledger, ID)
            with mock.patch.object(archive_copy, '_finish_pending', side_effect=[None, RuntimeError('interrupted')]):
                with self.assertRaises(RuntimeError):
                    archive_copy.expire_copy(target)
            with self.assertRaisesRegex(ValueError, 'interrupted'):
                archive_copy.verify_copy(target)
            self.assertEqual(archive_copy.expire_copy(target), 1)
            archive_copy.verify_copy(target)

    def test_stalled_transfer_has_total_deadline_and_reaps_process(self):
        proof, ledger = fixture()
        with tempfile.TemporaryDirectory() as base:
            parent = Path(base).resolve() / 'operator'
            parent.mkdir(mode=0o700)
            proc = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(10)'],
                                    stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                                    start_new_session=True)
            with self.assertRaises((ValueError, tarfile.ReadError)):
                archive_copy.copy_process(proc, parent / 'copy', proof, ledger, ID, timeout=.1)
            self.assertIsNotNone(proc.poll())


if __name__ == '__main__':
    unittest.main()
