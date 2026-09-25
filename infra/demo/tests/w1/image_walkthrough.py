#!/usr/bin/env python3
"""Run the existing manual/fake acceptance driver using the immutable FEAM binary.

The original driver copies FEAM to its temporary tree. W1 writable mounts are
noexec, so only that copy is replaced with a symlink to the installed image
binary. All walkthrough assertions, user choices and reader calls are unchanged.
"""
import os
from pathlib import Path
import sys

if len(sys.argv)!=5 or sys.argv[1]!='--mode' or sys.argv[2] not in ('manual','fake') or sys.argv[3]!='--output':
    raise SystemExit('only manual/fake walkthrough and explicit output are accepted')
sys.path.insert(0, '/opt/feam/workspace/scripts')
import tui_walkthrough
original=tui_walkthrough.shutil.copy2

def installed_binary(source, destination, *args, **kwargs):
    if Path(source).resolve()==Path('/usr/local/bin/feam'):
        Path(destination).symlink_to('/usr/local/bin/feam')
        return str(destination)
    return original(source,destination,*args,**kwargs)

tui_walkthrough.shutil.copy2=installed_binary
tui_walkthrough.main()
