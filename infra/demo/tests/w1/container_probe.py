#!/usr/bin/env python3
"""Bounded disruptive resource tests, inside the disposable W1 container only."""
import errno
import json
import os
from pathlib import Path
import socket
import subprocess
import time

cg=Path('/sys/fs/cgroup')
assert os.getuid()==1000
status=Path('/proc/self/status').read_text()
assert 'CapEff:\t0000000000000000' in status
assert 'NoNewPrivs:\t1' in status and 'Seccomp:\t2' in status
assert (cg/'memory.max').read_text().strip()=='536870912'
assert (cg/'memory.swap.max').read_text().strip()=='0'
assert (cg/'pids.max').read_text().strip()=='128'
quota,period=map(int,(cg/'cpu.max').read_text().split());assert quota/period==0.5
try:
    Path('/root-write-probe').write_text('denied')
    raise AssertionError('image writable')
except OSError as exc:
    assert exc.errno in (errno.EROFS,errno.EACCES)
assert sorted(p.name for p in Path('/sys/class/net').iterdir())==['lo']
with socket.socket() as sock:
    sock.settimeout(.5)
    assert sock.connect_ex(('1.1.1.1',443))!=0

def counters(name):
    return {k:int(v) for k,v in (line.split() for line in (cg/name).read_text().splitlines())}

# Prove writable socket storage cannot evade the workspace quota.
socket_root=Path('/run/feam-terminal')
probe=socket_root/'.w1-byte-probe'
try:
    with probe.open('xb') as target:
        try:
            target.write(b'x'*(2*1024**2));target.flush()
            raise AssertionError('socket tmpfs byte quota not enforced')
        except OSError as exc:
            assert exc.errno==errno.ENOSPC
finally:
    probe.unlink(missing_ok=True)
inodes=[]
try:
    for index in range(70):
        candidate=socket_root/('.w1-inode-'+str(index))
        try:
            with candidate.open('xb'): pass
            inodes.append(candidate)
        except OSError as exc:
            assert exc.errno==errno.ENOSPC
            break
    else: raise AssertionError('socket tmpfs inode quota not enforced')
finally:
    for candidate in inodes: candidate.unlink()

before=counters('memory.events')
# One capped child deliberately exceeds the cgroup limit; no host-scale fork or
# allocation test. The parent and terminal must survive for evidence collection.
child=subprocess.run(['python','-c','x=bytearray(600*1024*1024); print(len(x))'],capture_output=True)
after=counters('memory.events')
assert child.returncode!=0 and after['oom_kill']>before['oom_kill']
children=[]
pid_before=counters('pids.events')
try:
    for _ in range(140):
        try:
            children.append(subprocess.Popen(['/bin/sleep','10']))
        except OSError as exc:
            assert exc.errno==errno.EAGAIN
            break
    else:
        raise AssertionError('PID limit not enforced')
finally:
    for child in children: child.terminate()
    for child in children: child.wait()
assert counters('pids.events')['max']>pid_before['max']
cpu_before=counters('cpu.stat')
started=time.monotonic()
workers=[subprocess.Popen(['python','-c','import time\nt=time.monotonic()+8\nwhile time.monotonic()<t: pass']) for _ in range(2)]
for child in workers: assert child.wait()==0
cpu_after=counters('cpu.stat')
assert cpu_after['nr_throttled']>cpu_before['nr_throttled']
assert (cpu_after['usage_usec']-cpu_before['usage_usec'])/1e6 <= (time.monotonic()-started)*.65+1
Path('/workspace/persistence-proof').write_text('feam-w1-private-data\n')
print(json.dumps({'uid':os.getuid(),'read_only_image':True,'capabilities_dropped':True,'seccomp':True,'no_new_privileges':True,'network_none':True,'memory_oom_kills':after['oom_kill']-before['oom_kill'],'pids_denied':counters('pids.events')['max']-pid_before['max'],'cpu_throttled_periods':cpu_after['nr_throttled']-cpu_before['nr_throttled'],'swap_bytes':0,'socket_tmpfs_byte_limit':True,'socket_tmpfs_inode_limit':True}))
