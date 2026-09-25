import importlib.util
from pathlib import Path
import json
import os
import tempfile
import unittest

spec=importlib.util.spec_from_file_location('vault',Path(__file__).resolve().parents[2]/'scripts/operator_vault.py')
vault=importlib.util.module_from_spec(spec);spec.loader.exec_module(vault)

class VaultTests(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.root=Path(self.tmp.name).resolve();os.chmod(self.root,0o700)
  self.key=self.root/'key';vault.keygen(self.key)
  self.source=self.root/'input';vault.new_private(self.source,json.dumps({'private_roster':['synthetic@example.invalid'],'site':'stopped'}).encode())
  self.destination=self.root/'operator.enc'
 def test_encrypted_revisions_are_authenticated_and_not_replenished(self):
  self.assertEqual(vault.seal(self.source,self.destination,self.key,0)['revision'],1)
  self.assertNotIn(b'synthetic@example.invalid',self.destination.read_bytes())
  self.assertEqual(vault.decode(self.destination,self.key)['configuration']['site'],'stopped')
  with self.assertRaisesRegex(ValueError,'stale'):vault.seal(self.source,self.destination,self.key,0)
  vault.seal(self.source,self.destination,self.key,1)
  self.assertEqual(vault.decode(self.root/'operator.enc.revision-1',self.key)['revision'],1)
  self.assertEqual(vault.decode(self.destination,self.key)['revision'],2)
  b=self.destination.read_bytes();self.destination.write_bytes(b[:-1]+bytes([b[-1]^1]))
  with self.assertRaises(Exception):vault.decode(self.destination,self.key)
 def test_wrong_key_symlink_and_public_permissions_are_rejected(self):
  vault.seal(self.source,self.destination,self.key,0)
  second=self.root/'second-key';vault.keygen(second)
  with self.assertRaises(Exception):vault.decode(self.destination,second)
  linked=self.root/'linked';linked.symlink_to(self.key)
  with self.assertRaises(Exception):vault.key(linked)
  self.key.chmod(0o644)
  with self.assertRaisesRegex(ValueError,'owner-only'):vault.key(self.key)
 def test_private_export_never_overwrites_existing_input(self):
  with self.assertRaises(FileExistsError):vault.new_private(self.source,b'replaced')
  self.assertIn('private_roster',self.source.read_text())

if __name__=='__main__':unittest.main()
