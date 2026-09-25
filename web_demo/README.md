# Web demo development baseline

W0 supplies versioned schemas, examples and offline validation. No gateway,
controller, broker or collector is implemented yet. Follow the
[interface contract](../docs/ubuntu_web_dev_demo_contract.md) and
[workplan](../docs/ubuntu_web_dev_demo_workplan.md); Rust remains FEAM's authority.

From the repository root, with Python 3.13:

```bash
python3 -m venv /tmp/feam-web-w0-venv
/tmp/feam-web-w0-venv/bin/python -m pip install -r infra/demo/requirements-dev.lock
export PATH="/tmp/feam-web-w0-venv/bin:$PATH"
python infra/demo/scripts/check.py
python web_demo/validate.py settings infra/demo/examples/settings.json
python -m unittest discover -s web_demo/tests -v
```

These commands require no SSH inventory, provider key, model request or cloud
account. Tests exercise schema/semantic rejection, not runtime authentication,
transaction durability or cross-user isolation. The examples contain fake IDs
and cannot provision a host. Event payload schemas/redaction and actual SQLite
migrations arrive with the consuming W2/W5/W6 components. A schema-valid caller
claim is never authorization; service peers/capabilities must be verified.

Use [operator checks](../infra/demo/README.md) for the separate actual-host and
full-system VM gates. Add Go unit/race tests and browser negatives when those
services appear; do not report nonexistent application tests as passing.
