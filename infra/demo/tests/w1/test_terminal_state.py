"""Operator state-loss/reset admission tests without Docker or privileged effects."""
import importlib.util
import json
from pathlib import Path
from types import SimpleNamespace
import tempfile
import unittest
from unittest.mock import patch

spec=importlib.util.spec_from_file_location('terminal',Path(__file__).resolve().parents[2]/'scripts/w1_terminal.py')
terminal=importlib.util.module_from_spec(spec);spec.loader.exec_module(terminal)
CONFIG={'image_id':'sha256:'+'a'*64,'parent_uuid':'00000000-0000-0000-0000-000000000001'}

class OperatorStateTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name);self.state=self.root/'state.json'
        for slot in ('slot-01','slot-02'):(self.root/slot).mkdir()
        for name,value in [('ROOT',self.root),('STATE',self.state)]:
            active=patch.object(terminal,name,value);active.start();self.addCleanup(active.stop)
        active=patch.object(terminal.os,'geteuid',return_value=0);active.start();self.addCleanup(active.stop)

    def invoke(self,action,*args):
        with patch('sys.argv',['terminal',action,*args]):terminal.main()

    def test_start_never_initializes_missing_state(self):
        with patch.object(terminal,'read',return_value=CONFIG),patch.object(terminal,'docker') as docker:
            with self.assertRaisesRegex(ValueError,'state missing'):self.invoke('start')
            docker.assert_not_called()
        self.assertFalse(self.state.exists())

    def test_initialization_rejects_existing_container(self):
        with patch.object(terminal,'read',return_value=CONFIG),patch.object(terminal.subprocess,'run'),patch.object(terminal,'docker',return_value=SimpleNamespace(stdout='owned-container')):
            with self.assertRaisesRegex(ValueError,'containers exist'):self.invoke('initialize')
        self.assertFalse(self.state.exists())

    def test_initialization_preserves_unrecorded_slot_bytes(self):
        data=self.root/'slot-01'/'keep';data.write_text('preserve')
        with patch.object(terminal,'read',return_value=CONFIG),patch.object(terminal.subprocess,'run'),patch.object(terminal,'docker',return_value=SimpleNamespace(stdout='')):
            with self.assertRaisesRegex(ValueError,'contains bytes'):self.invoke('initialize')
        self.assertEqual(data.read_text(),'preserve');self.assertFalse(self.state.exists())

    def test_occupied_spare_does_not_stop_current_container(self):
        self.state.write_text('{}')
        state={'generation':2,'slot':'slot-02','retained':[{'generation':1,'slot':'slot-01'}]}
        with patch.object(terminal,'read',side_effect=[CONFIG,state]),patch.object(terminal,'docker') as docker:
            with self.assertRaisesRegex(ValueError,'spare occupied'):self.invoke('reset','--confirm-generation','2')
            docker.assert_not_called()

if __name__=='__main__':unittest.main()
