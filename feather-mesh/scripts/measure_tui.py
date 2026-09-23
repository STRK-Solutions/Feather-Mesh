#!/usr/bin/env python3
"""Release-build PTY startup/RSS/input measurements on disposable manifests."""
import argparse, json, os, platform, subprocess, tempfile, threading, time
from pathlib import Path
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from tui_walkthrough import ScreenSession
from test_tui_pty import WORKSPACE

class SlowRouter(BaseHTTPRequestHandler):
    def log_message(self,*args): pass
    def do_POST(self):
        self.rfile.read(int(self.headers['Content-Length']))
        time.sleep(2)
        body=('data: '+json.dumps({'choices':[{'delta':{'content':'Slow synthetic reply.'},'finish_reason':'stop'}],'usage':{'prompt_tokens':1,'completion_tokens':1,'cost':0}})+'\n\ndata: [DONE]\n\n').encode()
        self.send_response(200);self.send_header('Content-Type','text/event-stream');self.send_header('Content-Length',str(len(body)));self.end_headers()
        try:self.wfile.write(body)
        except BrokenPipeError:pass

def main():
    p=argparse.ArgumentParser();p.add_argument('--executable',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args();exe=a.executable.resolve();reports=[]
    with tempfile.TemporaryDirectory(prefix='feam-measure-') as tmp:
      root=Path(tmp);demo=root/'demo'
      subprocess.run([str(WORKSPACE/'scripts/tui_agent_stage1_demo.sh'),str(demo)],check=True,stdout=subprocess.DEVNULL,env=dict(os.environ,FEAM_EXECUTABLE=str(exe)))
      manifest=demo/'provider/serving/manifest.json';base=json.loads(manifest.read_text());version=base['products'][0]['versions'][0]
      for count in [3,1000,3000]:
        data=json.loads(json.dumps(base))
        if count>3:data['products'][0]['versions']=[dict(version,version=f'benchmark-{i:04}') for i in range(count-1)]
        manifest.write_text(json.dumps(data));size=manifest.stat().st_size
        start=time.monotonic();s=ScreenSession([str(exe),'--project',str(demo/'client with spaces'),'tui'],root/'state');s.until('versions;');startup=(time.monotonic()-start)*1000
        rss=[]
        def sample():
          result=subprocess.run(['ps','-o','rss=','-p',str(s.process.pid)],capture_output=True,text=True)
          if result.stdout.strip():rss.append(int(result.stdout.strip())/1024)
        sample();latencies=[]
        for _ in range(5):
          for key,marker in [('?','[Help]'),('\t','[Teams]')]:
            start=time.monotonic();os.write(s.master,key.encode())
            while marker not in s.text():s.pump(.001)
            latencies.append((time.monotonic()-start)*1000);sample()
        s.send('r');s.pump(.1);sample();s.send('q');s.finish()
        reports.append({'catalog_versions':count,'manifest_bytes':size,'startup_ms':startup,'sampled_rss_mib':rss,'input_response_ms':latencies})
      server=ThreadingHTTPServer(('127.0.0.1',0),SlowRouter);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
      config=root/'config/feam';config.mkdir(parents=True)
      config.joinpath('agent.toml').write_text(f'''schema_version=1
default_profile="slow"
[profiles.slow]
backend="router"
base_url="http://127.0.0.1:{server.server_port}"
model="synthetic"
api_key_env="FEAM_SYNTHETIC_KEY"
context_policy="metadata-only"
allow_user_text=true
max_context_chars=48000
''')
      manifest.write_text(json.dumps(base));s=ScreenSession([str(exe),'--project',str(demo/'client with spaces'),'tui','--agent','hosted'],root/'state',{'XDG_CONFIG_HOME':str(config.parent),'FEAM_SYNTHETIC_KEY':'synthetic-only'})
      s.until('versions;');s.send('aExplain direct reads\r');start=time.monotonic();os.write(s.master,b'?')
      while '[Help]' not in s.text():s.pump(.001)
      slow_latency=(time.monotonic()-start)*1000;s.send('s');s.send('q');s.finish();server.shutdown()
    report={'platform':platform.platform(),'machine':platform.machine(),'build':'cargo build --release -p mesh_cli --features agent-hosted','page_limit':25,'conditions':'local filesystem, warm OS cache; ps samples, not OS peak RSS; PTY 80x24; no inference process','catalogs':reports,'input_during_two_second_provider_delay_ms':slow_latency,'targets':{'rss_mib':100,'input_ms':100}}
    a.output.write_text(json.dumps(report,indent=2));print(json.dumps(report,indent=2))
if __name__=='__main__':main()
