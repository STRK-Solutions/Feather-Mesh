#!/usr/bin/env python3
"""Disposable manual/fake/live PTY walkthrough with explicit local user choices.
Requires pyte; it is an acceptance driver, never a production auto-approval mode.
"""
import argparse, codecs, json, os, subprocess, sys, shutil, tempfile, time
from pathlib import Path
import pyte
from test_tui_pty import Session, CLI, WORKSPACE

class ScreenSession(Session):
    def __init__(self,*args,**kwargs):
        self.screen=pyte.Screen(80,24);self.stream=pyte.Stream(self.screen);self.decoder=codecs.getincrementaldecoder('utf-8')('replace');self.offset=0;self.assistant_count=0
        super().__init__(*args,**kwargs)
    def pump(self,seconds=.15):
        super().pump(seconds)
        self.stream.feed(self.decoder.decode(self.raw[self.offset:]));self.offset=len(self.raw)
    def text(self): return '\n'.join(self.screen.display)
    def until(self,text,timeout=40):
        try: self.wait(lambda:text in self.text(),timeout)
        except AssertionError: raise AssertionError(f'Expected screen marker {text!r}; screen:\n{self.text()}') from None
    def command(self,text): self.send(':'+text+'\r')
    def ask(self,text):
        self.assistant_count+=1
        self.send('a'+text+'\r');self.until(f'Assistant result #{self.assistant_count} ',120)
    def review(self): self.until('y confirm')

def event(tool,args,id): return {'ToolCall':{'id':id,'name':tool,'arguments':args}}
def replay_turns(kind):
    if kind=='consumer':
        return [[event('catalog.search',{'text':'observations'},'search')],[{'TextDelta':'Choose v1 or v2?'}],
                [event('product.resolve',{'reference':'product://climate/observations','version':'v1'},'resolve')],[{'TextDelta':'Resolved the pinned v1 inventory.'}],
                [event('product.stage',{'handle':'$selected'},'stage-denied')],
                [event('product.stage',{'handle':'$selected'},'stage-confirmed')],
                [event('product.stage',{'handle':'$selected'},'stage-changed')]]
    return [[event('publication.validate',{'handle':'selected-draft','patch':{'contact':'demo@example.test'}},'patch')],
            [event('publication.publish',{'handle':'$selected'},'publish')],
            [event('product.withdraw',{'handle':'$selected'},'withdraw')]]

def main():
    p=argparse.ArgumentParser();p.add_argument('--mode',choices=['manual','fake','live'],required=True);p.add_argument('--output',type=Path,required=True);p.add_argument('--key-file',type=Path);args=p.parse_args()
    assert not args.output.exists(),'retain previous walkthrough records; choose a new output path'
    report={'mode':args.mode,'checks':[],'passed':False};started=time.monotonic();sessions=[]
    try:
      with tempfile.TemporaryDirectory(prefix='feam-walkthrough-') as temporary:
        root=Path(temporary);demo=root/'demo';state=root/'state';config=root/'config';config.joinpath('feam').mkdir(parents=True)
        executable=root/'feam';shutil.copy2(CLI,executable)
        subprocess.run([str(WORKSPACE/'scripts/tui_agent_stage1_demo.sh'),str(demo)],check=True,stdout=subprocess.DEVNULL,env=dict(os.environ,FEAM_EXECUTABLE=str(executable)))
        env={'XDG_CONFIG_HOME':str(config)}
        if args.mode=='live':
            assert args.key_file,'live requires a private credential file'
            env['FEAM_ROUTER_API_KEY']=args.key_file.read_text().strip()
            config.joinpath('feam/agent.toml').write_text('''schema_version = 1
default_profile = "acceptance"
[profiles.acceptance]
backend = "router"
base_url = "https://openrouter.ai/api/v1"
model = "deepseek/deepseek-v4.1-flash"
api_key_env = "FEAM_ROUTER_API_KEY"
context_policy = "synthetic-demo"
allow_user_text = true
max_context_chars = 48000
max_response_chars = 16000
max_output_tokens = 1024
max_cost_usd = 0.10
max_input_price = 0.14
max_output_price = 0.42
allowed_providers = ["deepinfra/fp8"]
allow_provider_fallbacks = false
''')
        def launch(project,kind):
            local=dict(env)
            if args.mode=='fake':
                replay=root/f'{kind}-replay.json';replay.write_text(json.dumps(replay_turns(kind)));local['FEAM_TUI_REPLAY']=str(replay)
            mode={'manual':'off','fake':'fake','live':'hosted'}[args.mode]
            s=ScreenSession([str(executable),'--project',str(project),'tui','--agent',mode],state,local);sessions.append(s);s.until('versions;');return s
        def collect_usage(session,project,kind):
            if args.mode=='manual': return
            session.command('usage');session.until('Assistant session usage')
            session.command('export usage.json');session.review();session.send('y');session.until('Saved')
            report.setdefault('usage',{})[kind]=json.loads((project/'.feam/exports/usage.json').read_text())
        consumer=demo/'client with spaces';provider=demo/'provider';s=launch(consumer,'consumer')
        assert not (consumer/'registry.db').exists();report['checks'].append('unrelated cwd and spaced path without registry')
        s.send('\t');s.until('unavailable');s.send('\t\t\t\t\t\t');s.pump()
        s.send('/observations\r');s.until('2 versions;');s.send('\r');s.until('manifest_revision');s.send('e');s.until('Python SDK');s.command('export table-example.txt');s.review();s.send('y');s.until('Saved')
        assert (consumer/'.feam/exports/table-example.txt').is_file();report['checks'].append('discovery, partial coverage, pinned metadata and export')
        def execute_example(name,assertions):
            text=(consumer/'.feam/exports'/name).read_text().split('Python SDK:\n',1)[1]
            subprocess.run([sys.executable,'-c',text+'\n'+assertions],check=True,cwd=root,env=dict(os.environ,FEAM_EXECUTABLE=str(executable)),capture_output=True)
        execute_example('table-example.txt', 'assert query.collect()["station_id"].to_list() == [1,2,3,4]')
        s.send('/temperature\r');s.until('1 versions;');s.send('e');s.until('Python SDK');s.command('export raster-example.txt');s.review();s.send('y');s.until('Saved')
        execute_example('raster-example.txt', 'with rasterio.open(product.assets[0].path) as check:\n    assert check.read(1, window=Window(0,0,2,2)).tolist() == [[7,8],[9,10]]')
        report['checks'].append('exported SDK examples execute externally with native Polars and known Rasterio pixels')
        s.send('/observations\r');s.until('2 versions;')
        if args.mode!='manual':
            s.ask('Find registered observations and ask me to select v1 or v2 before resolution.')
            s.ask('Select observations v1. Resolve product://climate/observations version v1 using product.resolve.')
            report['checks'].append('assistant clarification then explicit pinned resolution')
        destination=root/'staged data'
        def proposal(path, fresh_after_denial=False):
            s.command(f'stage product://climate/observations v1 "{path}"');s.review()
            if args.mode!='manual':
                s.send('h');s.until('Selected handle')
                prompt = ('I have selected a NEW local operation and explicitly request a fresh product.stage review for its current handle. The earlier denied operation stays denied; this is a new user request, not an automatic retry.' if fresh_after_denial else 'I have selected a new local operation. Open a fresh product.stage review using ONLY the currently selected handle. Do not reuse a prior denied or committed operation.')
                s.send('a'+prompt+'\r');s.review()
        proposal(destination);s.send('n');assert not destination.exists();report['checks'].append('denied stage produces no output')
        proposal(destination, fresh_after_denial=True);s.send('y');s.wait(lambda:destination.joinpath('part-000').exists());s.until('committed')
        receipt=json.loads(destination.with_name('staged data.feam-receipt.json').read_text());assert receipt['version']=='v1';assert len(receipt['assets'])==2
        report['checks'].append('confirmed stage receipt and exact two-asset inventory')
        changed=root/'changed output';proposal(changed);changed.write_text('preserved bytes');s.send('y');s.until('failed_before_commit');assert changed.read_text()=='preserved bytes';report['checks'].append('changed destination invalidates review')
        collect_usage(s,consumer,'consumer');s.send('q');s.finish()
        s=launch(provider,'producer');draft=json.loads(provider.joinpath('observations-v1.json').read_text());draft['version']='v3';draft['contact']='';path=provider/'draft with spaces.json';path.write_text(json.dumps(draft))
        s.command(f'draft load "{path}"');s.until('Draft loaded')
        s.command('draft validate');s.review();s.send('y');s.until('validation failed: contact')
        if args.mode!='manual':
            s.command('agent-draft');s.until('Draft handle')
            s.ask('Use publication.validate to propose contact="demo@example.test" for local handle selected-draft. I supply this contact. Do not publish yet.')
        else:s.command('draft set /contact \'"demo@example.test"\'')
        s.command('draft validate');s.review();s.send('y');s.until('validated metadata')
        if args.mode!='manual':
            s.send('h');s.until('Selected handle');s.send('aPropose publication.publish using the locally selected validated operation handle.\r');s.review()
        s.send('y');s.until('committed')
        manifest=json.loads(provider.joinpath('serving/manifest.json').read_text());assert any(v['version']=='v3' for p in manifest['products'] for v in p['versions']);report['checks'].append('incomplete draft, supplied contact patch, validated publication')
        s.command('withdraw product://foreign/observations v3 invalid');s.until('only local namespace')
        s.command('withdraw product://climate/observations v3 acceptance-complete');s.review()
        if args.mode!='manual':
            s.send('h');s.until('Selected handle');s.send('aPropose product.withdraw using the selected local handle.\r');s.review()
        s.send('y');s.until('committed')
        result=subprocess.run([str(executable),'--project',str(provider),'--format','json','resolve','product://climate/observations','--version','v3'],capture_output=True)
        assert result.returncode==5;assert b'withdrawn_version' in result.stderr;report['checks'].append('cross-namespace rejection and local withdrawal')
        collect_usage(s,provider,'producer');s.send('q');s.finish()
        outcomes=[]
        for file in state.glob('feam/tui/operations/*.json'):
            value=json.loads(file.read_text());outcomes.append({'operation_id':value['operation_id'],'operation':value['operation'],'state':value['state'],'reference':(value.get('result') or {}).get('reference'),'version':(value.get('result') or {}).get('version')})
        report['outcomes']=outcomes;report['passed']=True
    except Exception as error:
        report['failure']=str(error)
        raise
    finally:
        for session in sessions: session.close()
        report['elapsed_seconds']=time.monotonic()-started
        args.output.write_text(json.dumps(report,indent=2))
    print(json.dumps(report,indent=2))
if __name__=='__main__':main()
