# Bounded cloud and research archive proposal

Prepared 2026-09-25; prices checked against official provider pages. The owner
approved both bounded proposals. The private R2 bucket was created and its
retention/public-access settings verified; collector durability is still pending.
No paid compute has been created. Recipes are in
[Terraform modules](../../../infra/demo/terraform/README.md), separately locked
and tested with mock providers. Account-specific region availability, taxes
and an actual reviewed plan remain prerequisites.

## Cloud test

Propose DigitalOcean NYC3 for Ottawa proximity: the bundled 8-vCPU, 16-GiB
regular Droplet includes a 320-GiB OS/scratch disk and public IPv4 at
US$0.14286/hour, capped at $96/month. Add a separate 320-GiB service volume
at $32/month (approximately $0.048/hour). Total approximately $0.191/hour,
$128/month before tax. Stop still accrues resource charges; deletion of both
VM and volume is required. No reserved IP, snapshots or paid backups.
[Droplet/IPv4 pricing](https://www.digitalocean.com/pricing/droplets),
[volume pricing](https://www.digitalocean.com/pricing/volumes).

Request a maximum of **12 aggregate instance-hours across two sequential
create/test/delete cycles**, approximately $2.30 before tax, with a **US$5
authorization ceiling**. The operator monitors actual cost, stops work and
cleans up before this ceiling; it is not a provider-enforced hard billing cap.
The planned two 60-minute active-cohort tests fit within that window if setup
passes. Inference is accounted separately and needs its own finite approval.
Retained resources after a failed deletion are reported and reconciled, never
treated as deleted. No automatic replacement or cloud recovery is authorized.

Comparison: Hetzner's current US CPX41 compute alone is $141.49/month,
$0.2267/hour excluding IPv4 and extra service storage. European CX43 compute
is much cheaper at $18.49/month, $0.0296/hour, but requires added storage and
an unmeasured transatlantic terminal path. This is a cost comparison, not a
measured latency rejection. Prefer the bounded NYC3 test for the Ottawa cohort;
it is not claimed to be the world's cheapest available host.
[Current Hetzner prices](https://docs.hetzner.com/general/infrastructure-and-availability/price-adjustment/).

## Research archive

Propose a dedicated private Cloudflare R2 Standard bucket in eastern North
America with 30-day retention and no public domain. Initial archive admission
will cap retained application data at 1 GiB, with bounded five-minute/8-MiB
batches; stop capture visibly when the cap cannot be honored. Existing account
usage must be checked because the free tier is shared at account level.

R2 Standard includes 10 GB-month, one million Class A and ten million Class B
operations monthly; beyond those allowances, storage is $0.015/GB-month,
Class A $4.50/million and Class B $0.36/million, with free egress. Under the
proposed small demo limits it is expected to fit the free allowance if that
allowance remains available. Billing is not a hard cap. Request **US$1 total
for this bounded archive validation**, including retrieval after disposable
source removal; raise no automatic allowance.
[R2 prices](https://developers.cloudflare.com/r2/pricing/).

Saif is the sole reviewer and Phase 2 owner. The contact and private policy
are in owner-only operator input. No real participant capture/export occurs
until credentials, archive verification and deletion handling are ready.
