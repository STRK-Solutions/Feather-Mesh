# Private operator state

The operator keeps enrollment/policy, external Terraform state and the project
spending ledger outside Ubuntu/cloud compute. Disposable service databases never
reconstruct the project allowance. The current implementation uses a Mac-held
AES-256-GCM policy vault, the broker's encrypted integer spending ledger and three
separate Terraform states. The original owner-supplied ignored inputs are retained.

`operator_vault.py` takes only owner-only regular files beneath existing absolute
mode-0700 directories. It authenticates the versioned JSON, rejects stale writer
revisions, retains prior encrypted revisions, fsyncs replacements and refuses to
overwrite a plaintext export. Key generation is explicit and never replaces an
existing key. Test with:

```sh
python -m unittest discover -s infra/demo/tests/operator -v
```

Keep the encryption key outside the checkout and separately recoverable. The
current local key is under the owner's private `~/.config/feam-demo/` directory;
operator state is ignored under `.local/demo-deployment/operator-state/`. These
paths are operator inputs, not paths available to participant processes. A clean
clone intentionally contains neither the real enrollment nor its key.

```sh
python infra/demo/scripts/operator_vault.py keygen --key /private/operator-keys/vault.key
python infra/demo/scripts/operator_vault.py seal --input /private/operator-inputs/policy.json --output /private/operator-state/policy.enc --key /private/operator-keys/vault.key --expected-revision 0
python infra/demo/scripts/operator_vault.py verify --input /private/operator-state/policy.enc --key /private/operator-keys/vault.key
python infra/demo/scripts/operator_vault.py open --input /private/operator-state/policy.enc --output /private/operator-inputs/current-policy.json --key /private/operator-keys/vault.key
```

The next seal requires the verified current revision. An `open` destination must
not exist. Remove generated plaintext only after the consumer has completed; do
not remove the owner's original inputs or an encrypted prior revision as part of
routine converge. Never put plaintext or keys in Terraform values, build contexts,
source archives, command output or research exports.

Use [project-budget](../../web_demo/BROKER.md#operator-project-ledger) for the
separate signed allocation and conservative spend reconciliation. Initial known
charges must be supplied explicitly; missing history is not zero spending. A
broker drain receipt containing unknown requests cannot release their reservation.
Use [separate Terraform lifecycles](terraform/README.md) for external resources.

Current owner policy: on-demand operation, 30-day research retention, Saif as sole
export reviewer and Phase 2 owner. The support address is in the private policy.
No fixed service hours, new participant consent or SLM training are introduced.
Archive credentials, recovery ownership, current allowance and a successful
retrieval after disposable-compute deletion remain handoff checks.
