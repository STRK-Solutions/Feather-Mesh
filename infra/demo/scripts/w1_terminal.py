#!/usr/bin/env python3
"""Operator-only W1 finite slot launcher; production lifecycle controller is W3."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import time

ROOT = Path('/home/feam-service-data')
CONFIG = Path('/etc/feam/w1-terminal.json')
STATE = ROOT/'w1-terminal-state.json'
SOCKET = Path('/run/feam-terminal-w1/slot-01')
LABEL = 'feam.scope=w1-private'


def docker(*args, check=True):
    return subprocess.run(['runuser', '-u', 'feam-runner', '--', 'env',
        'XDG_RUNTIME_DIR=/run/user/2101', 'DOCKER_HOST=unix:///run/user/2101/feam-docker.sock',
        'docker', *args], check=check, capture_output=True, text=True)


def read(path):
    if path.is_symlink() or path.stat().st_uid != 0 or path.stat().st_mode & 0o022:
        raise ValueError('operator record must be root-owned and non-writable')
    return json.loads(path.read_text())


def write_state(state):
    tmp = STATE.with_suffix('.new')
    with tmp.open('x') as output:
        os.chmod(tmp, 0o600)
        json.dump(state, output)
        output.flush(); os.fsync(output.fileno())
    os.replace(tmp, STATE)
    fd=os.open(ROOT,os.O_DIRECTORY)
    try: os.fsync(fd)
    finally: os.close(fd)


def inspect(name):
    result=docker('container','inspect',name,check=False)
    if result.returncode:
        # Do not treat daemon failure as an absent resource.
        docker('info')
        if 'No such container' not in result.stderr and 'No such object' not in result.stderr:
            raise ValueError('container outcome unknown')
        return None
    value=json.loads(result.stdout)[0]
    if value['Config']['Labels'].get('feam.scope') != 'w1-private':
        raise ValueError('refusing an unowned container')
    return value


def start(config,state):
    subprocess.run(['/usr/local/sbin/feam-w1-storage','verify','--parent-uuid',config['parent_uuid']],check=True)
    if not os.path.ismount(SOCKET) or subprocess.check_output(['findmnt','-n','-o','FSTYPE','--mountpoint',str(SOCKET)],text=True).strip()!='tmpfs':
        raise ValueError('bounded socket mount missing')
    name='feam-w1-g'+str(state['generation'])
    current=inspect(name)
    if current:
        if current['Image'] != config['image_id'] or current['Config']['Labels'].get('feam.slot') != state['slot']:
            raise ValueError('container identity mismatch')
        if not current['State']['Running']: docker('start',name)
    else:
        docker('run','--detach','--name',name,'--label',LABEL,'--label','feam.slot='+state['slot'],
            '--restart','no','--init','--user','1000:1000','--read-only','--cap-drop','ALL',
            '--security-opt','no-new-privileges','--network','none','--memory','512m',
            '--memory-swap','512m','--cpus','0.5','--pids-limit','128',
            '--ulimit','nofile=1024:1024','--shm-size','16m',
            '--tmpfs','/tmp:rw,nosuid,nodev,noexec,size=64m,mode=1777',
            '--mount','type=bind,src='+str(ROOT/state['slot'])+',dst=/workspace',
            '--mount','type=bind,src='+str(SOCKET)+',dst=/run/feam-terminal',
            '--log-driver','local','--log-opt','max-size=1m','--log-opt','max-file=2',config['image_id'])
    print(json.dumps({'container':name,'generation':state['generation'],'slot':state['slot']}))


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('action',choices=['initialize','start','stop','status','reset'])
    p.add_argument('--confirm-generation',type=int)
    a=p.parse_args()
    if os.geteuid()!=0: p.error('operator root required')
    config=read(CONFIG)
    if set(config)!={'image_id','parent_uuid'} or not re.fullmatch(r'sha256:[0-9a-f]{64}',config['image_id']):
        raise ValueError('invalid approved immutable image configuration')
    if a.action=='initialize':
        if STATE.exists() or STATE.is_symlink():
            raise ValueError('operator state already exists')
        subprocess.run(['/usr/local/sbin/feam-w1-storage','verify','--parent-uuid',config['parent_uuid']],check=True)
        if docker('ps','--all','--quiet','--filter','label='+LABEL).stdout.strip():
            raise ValueError('owned containers exist; missing state requires reconciliation')
        for slot in ('slot-01','slot-02'):
            if any(p.name!='lost+found' for p in (ROOT/slot).iterdir()):
                raise ValueError('slot contains bytes; initialization is not recovery')
        write_state({'generation':1,'slot':'slot-01','retained':[]})
        print('initialized empty W1 private slice');return
    if not STATE.exists():
        raise ValueError('operator state missing; explicit initialization or reconciliation required')
    state=read(STATE)
    if state['slot'] not in {'slot-01','slot-02'} or type(state['generation']) is not int or state['generation']<1:
        raise ValueError('invalid finite slot state')
    name='feam-w1-g'+str(state['generation'])
    if a.action=='status':
        print(json.dumps({'state':state,'container':inspect(name)}));return
    if a.action == 'reset' and state['retained']:
        raise ValueError('spare occupied; W1 never deletes retained bytes')
    if a.action in ('stop','reset'):
        if a.action=='reset' and a.confirm_generation!=state['generation']:
            raise ValueError('reset requires the exact current generation')
        existing=inspect(name)
        if existing and existing['State']['Running']: docker('stop','--time','15',name)
    if a.action=='reset':
        spare='slot-02' if state['slot']=='slot-01' else 'slot-01'
        subprocess.run(['/usr/local/sbin/feam-w1-storage','verify','--parent-uuid',config['parent_uuid']],check=True)
        if any(p.name!='lost+found' for p in (ROOT/spare).iterdir()):
            raise ValueError('reset spare is not empty')
        state={'generation':state['generation']+1,'slot':spare,'retained':[state]}
        write_state(state) # Persistent intent before initialization; failure is reconciled, never reset again.
    if a.action in ('start','reset'): start(config,state)


if __name__=='__main__':
    # Serialize all operator lifecycle operations; no arbitrary path or command.
    import fcntl
    with open('/run/feam-w1-lifecycle.lock','w') as lock:
        fcntl.flock(lock,fcntl.LOCK_EX)
        main()
