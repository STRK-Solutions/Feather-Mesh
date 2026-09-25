#!/usr/bin/env python3
"""Private browser proof through the SSH-forwarded operator terminal.

Never accepts participant identities, real provider keys or live inference.
The browser credential stays in an owner-only local file and is not output.
"""
import argparse
import json
from pathlib import Path
import shlex
import tempfile
import time
import uuid
import pyte
from playwright.sync_api import sync_playwright

p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--credential-file',type=Path,required=True)
p.add_argument('--output',type=Path,required=True)
p.add_argument('--url',default='http://127.0.0.1:18771')
a=p.parse_args()
if not a.url.startswith('http://127.0.0.1:') or a.credential_file.stat().st_mode & 0o077:
    p.error('loopback private URL and owner-only credential file required')
with sync_playwright() as driver:
    browser=driver.chromium.launch()
    context=browser.new_context(http_credentials={'username':'operator','password':a.credential_file.read_text().strip()},viewport={'width':1280,'height':800})
    page=context.new_page()
    frames=[]
    screen=pyte.Screen(160,60)
    stream=pyte.Stream(screen)
    def received(payload):
        value=payload.decode(errors='replace') if isinstance(payload,bytes) else payload
        if value.startswith('0'):
            stream.feed(value[1:])
            # ttyd frames contain terminal cursor updates, not complete lines.
            # Match the reconstructed screen after each output frame.
            frames.append('\n'.join(screen.display))
    def sent(payload):
        value=payload.decode(errors='replace') if isinstance(payload,bytes) else payload
        if value.startswith('1'):
            size=json.loads(value[1:])
            screen.resize(lines=size['rows'],columns=size['columns'])
    def watch(ws):
        ws.on('framereceived',received)
        ws.on('framesent',sent)
    page.on('websocket',watch)
    def until_frame(marker, start=0):
        deadline=time.monotonic()+120
        while time.monotonic()<deadline and marker not in ''.join(frames[start:]):
            page.wait_for_timeout(250)
        if marker not in ''.join(frames[start:]):
            with tempfile.NamedTemporaryFile(prefix='feam-w1-browser-failure-',suffix='.png',delete=False) as failure:
                page.screenshot(path=failure.name)
            raise AssertionError(f'browser terminal missing {marker!r}; screenshot={failure.name}')
        print('Browser observed '+marker,flush=True)
    page.goto(a.url,wait_until='networkidle')
    until_frame('versions;')
    page.locator('.xterm-helper-textarea').focus()
    page.keyboard.type(':resolve product://climate/observations v1',delay=20)
    page.keyboard.press('Enter')
    until_frame('part-000')
    # The operator types a fixed local replay in the sandbox shell. This uses
    # the existing FEAM fake provider, never gateway identity or live inference.
    offset=len(frames)
    page.keyboard.type('q')
    until_frame('Private sandbox shell',offset)
    screen.reset()
    replay='/tmp/w1-browser-replay-'+uuid.uuid4().hex+'.json'
    program=('import sys,json;sys.path.insert(0,"/opt/feam/workspace/scripts");'
             'from tui_walkthrough import replay_turns;'
             f'open({replay!r},"x").write(json.dumps(replay_turns("consumer")))')
    command=('python -c '+shlex.quote(program)+' && FEAM_TUI_REPLAY='+shlex.quote(replay)
             +" feam tui --project '/workspace/demo/client with spaces' --agent fake")
    offset=len(frames)
    page.keyboard.type(command)
    page.keyboard.press('Enter')
    until_frame('versions;',offset)
    offset=len(frames)
    page.keyboard.type('aFind registered observations and ask me to select v1 or v2 before resolution.',delay=20)
    page.keyboard.press('Enter')
    until_frame('Assistant result #1 ',offset)
    until_frame('Choose v1 or v2?',offset)
    offset=len(frames)
    page.keyboard.type('aSelect observations v1. Resolve product://climate/observations version v1 using product.resolve.',delay=20)
    page.keyboard.press('Enter')
    until_frame('Assistant result #2 ',offset)
    until_frame('Resolved the pinned v1 inventory.',offset)
    page.keyboard.type('q')
    # Browser origin negatives use real fetch/WS rather than header-only mocks.
    denied=context.request.get(a.url+'/?arg=sh')
    assert denied.status==400
    anonymous=browser.new_context()
    assert anonymous.request.get(a.url).status==401
    anonymous.close()
    a.output.parent.mkdir(parents=True,exist_ok=True)
    a.output.write_text(json.dumps({'browser':'chromium','transport':'private SSH to Unix socket','live_tui':True,'manual_resolve':True,'fake_clarification_and_pinned_resolution':True,'query_args_rejected':True,'anonymous_denied':True,'live_inference':False},indent=2)+'\n')
    context.close();browser.close()
print('PASS: actual browser WebSocket FEAM manual/fake terminal and admission negatives')
