#!/usr/bin/env python3
"""Explicit opt-in launcher; loads a private key without exposing it in argv."""
import argparse
import json
import os
from pathlib import Path
import subprocess


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--live',action='store_true',required=True)
    parser.add_argument('--key-file',type=Path,required=True)
    parser.add_argument('--profile',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True)
    parser.add_argument('--budget-usd',type=float,required=True)
    parser.add_argument('--start-index',type=int,default=0)
    parser.add_argument('--corpus',type=Path)
    parser.add_argument('--limit',type=int)
    parser.add_argument('--executable',type=Path,default=Path('target/debug/examples/stage1_eval'))
    args=parser.parse_args()
    if not 0 < args.budget_usd <= 19.98: parser.error('budget must fit the remaining recorded authorization')
    if args.output.exists(): parser.error('use a new output path to retain every attempt')
    profile=json.loads(args.profile.read_text())
    key=args.key_file.read_text().strip()
    if not key or any(c.isspace() for c in key): parser.error('credential file must contain one key only')
    env=dict(os.environ);env[profile['api_key_env']]=key
    command=[str(args.executable.resolve()),'--live','--profile',str(args.profile.resolve()),'--output',str(args.output.resolve()),'--budget-usd',str(args.budget_usd),'--start-index',str(args.start_index)]
    if args.corpus: command += ['--corpus',str(args.corpus.resolve())]
    if args.limit is not None: command += ['--limit',str(args.limit)]
    return subprocess.call(command,env=env)

if __name__=='__main__': raise SystemExit(main())
