# W7 private services: local implementation evidence

Recorded 2026-09-25 on the operator Mac. No service role was applied to Ubuntu by
this check, no public route was exposed, and no provider request was made.

Implemented artifacts: `infra/demo/ansible/services.yml`, the participant branch
of `roles/services`, separate service/resource/tmpfs/boot-layout/health units,
`services_runtime.py`, six offline tests, and `infra/demo/W7.md`. The W1 branch
remains separately selected by its original playbook. Default participant input
does not start/enable applications, initialize/migrate state or select live
inference. Root-owned runtime scope binds configuration and artifact hashes.

Checks passed using the locked Python/Ansible development environment:

```sh
python -m unittest discover -s infra/demo/tests/w7 -p test_services.py -v
ansible-playbook --syntax-check infra/demo/ansible/services.yml
ansible-playbook --syntax-check infra/demo/ansible/deploy-w1.yml
ansible-lint --offline infra/demo/ansible/services.yml infra/demo/ansible/deploy-w1.yml infra/demo/ansible/roles/services
git diff --check
bash -n infra/demo/scripts/native_web_build.sh infra/demo/images/terminal/launcher.sh
```

Six tests passed. Ansible lint processed 15 files and passed its production
profile. Syntax checks use no inventory connection; their expected empty-demo
inventory warning is not target evidence. Initial lint failures were seven
role-default variable prefix errors and one long YAML line; moving public
defaults into the explicit participant playbook and wrapping the line resolved
them. No rule was suppressed.

Tests reject changed configuration scope, traversal paths, wrong authority UIDs,
cross-workspace endpoints, missing live credential scope and invalid research
roles. A tampered collector policy fails before root ACL mutation. Rendered units
have explicit isolation/resource/boot settings and no implicit initialization.
The read-only health check leaves intentionally inactive services inactive.

Broker integration additionally passed focused Go race tests, vet, the existing
Rust HTTPS/private-Unix synthetic probe, queued/active cancellation, activity
lease failure, aggregate-status peer authorization, durable drain and restart
accounting. A service-level private Unix test checks bounded shutdown and receipt
writing for both active and already-paused runs. The project/run allocation is
never replenished by restart; unknown reservations remain charged to exposure.

Still required: actual Ubuntu systemd/ACL/mount/cgroup evidence, fake streams
across multiple isolated workspace identities, component restart/boot checks,
fresh and second converge, migration/rollback scope, desired-stop behavior,
private receipt archival and verified teardown gates. Actual target and cloud
acceptance remain open in the workplan; these local checks do not close them.

The added `site_lifecycle.py` operator workflow has fourteen synthetic tests:
durable pre-effect intent; exact interrupted-stop acknowledgment; no unknown
provider replay/allocation; archive failures preserving stopped intent and costs;
fresh verification invalidating an old preparation; pinned project binding;
owner-reviewed cost closure; missing prior archive/deletion/revocation rejection;
new-run artifact preparation preserving old DBs and the original project ceiling;
encrypted vault status with no secret output; subprocess output/deadline bounds.
No SSH or host/cloud mutation is performed by these tests. The next-run manifest
serialization is checked against locked Ansible's actual filter implementation.
Its first test attempt hit Ansible's default home cache restriction; scoping the
test to its own temporary Ansible directories resolved it without escalation.

Controller `status` now emits a consistent operator-only lifecycle snapshot;
its focused race test passed. Collector archive verify/export/import belongs to
the collector implementation and must be included in the final native snapshot;
an earlier approved native snapshot is baseline evidence only. The orchestration
runbook is `infra/demo/SITE_LIFECYCLE.md`. Real external revocation/provider
verification remains an explicit owner-evidence gate, and the script implements
teardown preparation only, never cloud deletion.
