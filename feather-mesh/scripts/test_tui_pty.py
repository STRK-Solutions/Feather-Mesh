"""Actual terminal input, resize, signals and unwind restoration (offline)."""
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import signal
import struct
import subprocess
import tempfile
import termios
import time
import unittest

WORKSPACE = Path(__file__).resolve().parents[1]
CLI = Path(os.environ.get('FEAM_EXECUTABLE', WORKSPACE/'target/debug/mesh_cli')).resolve()
PROBE = Path(os.environ.get('FEAM_TERMINAL_PROBE', WORKSPACE/'target/debug/examples/terminal_probe')).resolve()

class Session:
    def __init__(self, args, state, env=None):
        self.master, self.slave = pty.openpty()
        fcntl.ioctl(self.slave, termios.TIOCSWINSZ, struct.pack('HHHH', 24, 80, 0, 0))
        self.initial = termios.tcgetattr(self.slave)
        self.raw = b''
        environment = dict(os.environ, TERM='xterm-256color', NO_COLOR='1', XDG_STATE_HOME=str(state))
        if env: environment.update(env)
        self.process = subprocess.Popen(args, stdin=self.slave, stdout=self.slave, stderr=self.slave, cwd='/tmp', env=environment)
    def pump(self, seconds=.15):
        end=time.monotonic()+seconds
        while time.monotonic()<end:
            ready,_,_=select.select([self.master],[],[],min(.03,max(0,end-time.monotonic())))
            if ready:
                try: self.raw += os.read(self.master,65536)
                except OSError: break
    def send(self,text):
        os.write(self.master,text.encode());self.pump()
    def wait(self, predicate, timeout=8):
        start=time.monotonic()
        while not predicate():
            if time.monotonic()-start>timeout: raise AssertionError('terminal action did not reach its expected state')
            self.pump(.05)
    def finish(self):
        self.wait(lambda:self.process.poll() is not None)
        self.pump()
        assert b'\x1b[?1049h' in self.raw, 'alternate screen not entered'
        assert b'\x1b[?1049l' in self.raw, 'alternate screen not restored'
        restored=termios.tcgetattr(self.slave)
        # macOS sets this kernel input-retype flag when restoring canonical mode.
        restored[3] &= ~getattr(termios,'PENDIN',0)
        self.initial[3] &= ~getattr(termios,'PENDIN',0)
        assert restored==self.initial, 'terminal attributes not restored'
        os.close(self.master);os.close(self.slave)
    def close(self):
        if self.process.poll() is None:
            self.process.kill();self.process.wait()
        for fd in (self.master,self.slave):
            try: os.close(fd)
            except OSError: pass

class TerminalTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory(prefix='feam pty ');self.root=Path(self.temp.name)
        self.sessions=[]
    def tearDown(self):
        for session in self.sessions: session.close()
        self.temp.cleanup()
    def launch(self,args,env=None):
        session=Session(args,self.root/'private-state',env);self.sessions.append(session);session.pump(.3);return session
    def test_keyboard_resize_explicit_init_and_clean_exit(self):
        project=self.root/'new project with spaces'
        s=self.launch([str(CLI),'--project',str(project),'tui','--agent','off'])
        self.assertFalse((project/'.feam/project.toml').exists())
        s.send(':init climate serving\r');s.wait(lambda:(project/'.feam/project.toml').exists())
        for rows,cols in [(40,120),(20,60),(24,80)]:
            fcntl.ioctl(s.slave,termios.TIOCSWINSZ,struct.pack('HHHH',rows,cols,0,0));s.process.send_signal(signal.SIGWINCH);s.send('?\t')
        s.send('q');s.finish();self.assertEqual(s.process.returncode,0)
        self.assertFalse((project/'registry.db').exists())
    def test_signals_restore_terminal(self):
        for sig in [signal.SIGINT,signal.SIGTERM,signal.SIGHUP]:
            s=self.launch([str(CLI),'--project',str(self.root/f'project-{sig}'),'tui'])
            s.wait(lambda:b'\x1b[?1049h' in s.raw)
            s.process.send_signal(sig);s.finish()
    def test_error_and_panic_restore_terminal(self):
        for mode in ['error','panic']:
            s=self.launch([str(PROBE),mode]);s.finish();self.assertNotEqual(s.process.returncode,0)
    def test_missing_hosted_configuration_keeps_manual_controls(self):
        s=self.launch([str(CLI),'--project',str(self.root/'manual'),'tui','--agent','hosted'],{'XDG_CONFIG_HOME':str(self.root/'absent-config')})
        s.send('a?');s.send('q');s.finish();self.assertEqual(s.process.returncode,0)
    def test_fake_assistant_completes_without_an_extra_keypress(self):
        replay=self.root/'replay.json'
        response='LONG RESPONSE START\n'+'\n'.join(f'assistant line {i:03d}' for i in range(80))+'\nREPLAY RESPONSE ARRIVED'
        # Unit variant events are encoded as JSON strings by serde.
        replay.write_text(json.dumps([[{'TextDelta':response},'Finished']]))
        import codecs
        import pyte
        project=self.root/'fake'
        subprocess.run([str(CLI),'--project',str(project),'init','--namespace','climate','--serving-dir','serving'],check=True,capture_output=True)
        s=self.launch([str(CLI),'--project',str(project),'tui','--agent','fake'],{'FEAM_TUI_REPLAY':str(replay)})
        screen=pyte.Screen(80,24);stream=pyte.Stream(screen);decoder=codecs.getincrementaldecoder('utf-8')('replace');offset=0
        def visible(marker):
            nonlocal offset
            stream.feed(decoder.decode(s.raw[offset:]));offset=len(s.raw)
            return marker in '\n'.join(screen.display)
        s.wait(lambda:visible('versions;'))
        s.send('aShow replay\r')
        s.wait(lambda:visible('REPLAY RESPONSE ARRIVED'))
        self.assertIn('Assistant', screen.display[4])
        s.send('?');s.wait(lambda:visible('Commands (quote paths with spaces)'))
        self.assertNotIn('Versions', '\n'.join(screen.display))
        s.send('\t\t\t\t');s.wait(lambda:visible('REPLAY RESPONSE ARRIVED'))
        s.send('\x1b[5~'*10);s.wait(lambda:visible('LONG RESPONSE START'))
        s.send('\x1b[6~'*10);s.wait(lambda:visible('REPLAY RESPONSE ARRIVED'))
        s.send('q');s.finish()

if __name__=='__main__': unittest.main()
