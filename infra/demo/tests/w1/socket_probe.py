#!/usr/bin/env python3
"""Run under the chosen host identity; output contains no terminal contents."""
import argparse
import socket

p=argparse.ArgumentParser(description=__doc__)
p.add_argument('socket')
p.add_argument('--expect',choices=['allow','deny'],required=True)
a=p.parse_args()
with socket.socket(socket.AF_UNIX) as sock:
    sock.settimeout(3)
    try:
        sock.connect(a.socket)
        allowed=True
    except PermissionError:
        allowed=False
assert allowed==(a.expect=='allow'), 'unexpected Unix socket access'
print('PASS: socket access '+a.expect)
