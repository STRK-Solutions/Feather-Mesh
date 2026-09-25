"""Offline semantic capture through the actual TUI and a private Unix fixture."""
import json
from pathlib import Path
import socketserver
import tempfile
import threading
import unittest
import uuid
from http.server import BaseHTTPRequestHandler

from test_tui_pty import CLI
from tui_walkthrough import ScreenSession


class CaptureTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='fc-', dir='/tmp')
        self.root = Path(self.temp.name)
        self.events = []
        self.retries = []
        self.gap = False
        self.sessions = []
        fixture = self

        class Handler(BaseHTTPRequestHandler):
            protocol_version = 'HTTP/1.1'

            def log_message(self, *args):
                pass

            def do_POST(self):
                if self.path != '/v1/events' or self.headers.get('Authorization') != 'Bearer ' + 'a' * 43:
                    self.send_error(403)
                    return
                length = int(self.headers.get('Content-Length', '0'))
                if not 0 < length <= 65536:
                    self.send_error(400)
                    return
                event = json.loads(self.rfile.read(length))
                existing = next((old for old in fixture.events if old['event_id'] == event['event_id']), None)
                if existing is None:
                    fixture.events.append(event)
                else:
                    fixture.retries.append((existing, event))
                body = json.dumps({'event_id': event['event_id'], 'sequence': event['sequence'],
                                   'status': 'durable', 'gap': fixture.gap}).encode()
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(body)))
                self.end_headers()
                self.wfile.write(body)

        self.server = socketserver.UnixStreamServer(str(self.root / 'events.sock'), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        token = self.root / 'token'
        token.write_text('a' * 43)
        token.chmod(0o640)
        config = dict(socket=str(self.root / 'events.sock'), capability_file=str(token),
                      generation=1, synthetic=True)
        for key in ['deployment_id', 'workspace_id', 'participant_id']:
            config[key] = str(uuid.uuid4())
        for key in ['software_sha256', 'profile_sha256', 'dataset_sha256']:
            config[key] = 'a' * 64
        self.config = self.root / 'event.json'
        self.config.write_text(json.dumps(config))
        self.config.chmod(0o640)

    def tearDown(self):
        for session in self.sessions:
            session.close()
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)
        self.temp.cleanup()

    def launch(self):
        project = self.root / 'private project'
        session = ScreenSession([str(CLI), '--project', str(project), 'tui', '--agent', 'off'],
                          self.root / 'state', {'FEAM_EVENT_CONFIG': str(self.config)})
        self.sessions.append(session)
        session.until('recording gap' if self.gap else 'recording structured events')
        session.pump(.2)
        return session, project

    def test_review_edit_denial_approval_and_typed_outcome(self):
        session, project = self.launch()
        session.send(':init climate serving\r')
        session.wait(lambda: (project / '.feam/project.toml').exists())
        session.pump(.2)
        session.send(':draft new table\r')
        session.send(':draft set /description \'"private.person@example.invalid /Users/private/raw.parquet"\'\r')
        session.send(':draft save denied.json\r')
        session.wait(lambda: any(e['payload']['decision'] == 'presented' for e in self.events))
        session.send('n')
        self.assertFalse((project / '.feam/drafts/denied.json').exists())
        session.send(':draft save approved.json\r')
        session.pump(.3)
        session.send('y')
        session.wait(lambda: (project / '.feam/drafts/approved.json').exists())
        session.send('q')
        session.finish()
        self.assertEqual(session.process.returncode, 0)
        decisions = {e['payload']['decision'] for e in self.events}
        self.assertTrue({'started', 'submitted', 'edited', 'presented', 'denied', 'approved', 'cancelled'} <= decisions)
        self.assertTrue(any(e['kind'] == 'outcome' and e['payload']['outcome'] == 'committed' for e in self.events))
        self.assertEqual([e['sequence'] for e in self.events], list(range(1, len(self.events) + 1)))
        encoded = json.dumps(self.events)
        for private in ['private.person', '/Users/private', 'raw.parquet', 'approved.json', 'denied.json']:
            self.assertNotIn(private, encoded)
        self.assertTrue(all(e['trust'] == 'client_reported' for e in self.events))
        self.assertTrue(all(old == retry for old, retry in self.retries))

    def test_collector_gap_pauses_mutation_and_restores_terminal(self):
        self.gap = True
        session, project = self.launch()
        session.send(':init climate serving\r')
        session.pump(.3)
        self.assertFalse((project / '.feam/project.toml').exists())
        import pyte
        screen = pyte.Screen(80, 24)
        pyte.Stream(screen).feed(session.raw.decode('utf-8', errors='replace'))
        self.assertIn('recording gap', '\n'.join(screen.display))
        session.send('q')
        session.finish()
        self.assertNotEqual(session.process.returncode, 0)


if __name__ == '__main__':
    unittest.main()
