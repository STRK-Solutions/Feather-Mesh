# On-demand demo: domain and cloud shortlist

Researched 2026-09-24. This is a public-price comparison, not a purchase authorization, checkout quote, reservation, or deployed benchmark. The [design](ubuntu_web_dev_demo_design.md) and [workplan](ubuntu_web_dev_demo_workplan.md) own requirements and status.

**Current selection, updated 2026-09-25:** the owner switched the demo back to the purchased `613202690.xyz` domain and confirmed Cloudflare Active status. Use `https://feam.613202690.xyz`; public DNS returns its assigned Cloudflare nameservers. This supersedes the temporary `demo.saifshaikh.ca` selection. The prices and alternative names below are retained research, not outstanding purchase requirements. Demo DNS routes, HTTPS, Tunnel and Access still need configuration and verification.

## Recommended direction

Use the existing Ubuntu host when convenient. For paid runs, start with an hourly instance in eastern North America, comparing DigitalOcean New York/Toronto and a nearby OVHcloud offer. The Ottawa users interact with a live terminal, so the likely latency benefit is worth considering alongside the small cost of a short run. Actual routing through Cloudflare, host contention and model-provider latency must be measured; geography is an engineering inference, not a latency result. European Hetzner is a lower compute-price comparison, not the default recommendation.

Create a fresh demo from IaC, current private configuration and pinned seeds; delete disposable compute/storage/IPs afterward. Keep research traces, reviewed exports, enrollment/policy inputs and project spending outside that deletion scope. No warm standby, automatic recovery, independent recovery runner, or replicated demo DB is needed.

## Domain candidates

### Portal and admin addresses

Use these first-level sibling hostnames in the active `613202690.xyz` zone:

| Purpose | Proposed address |
| --- | --- |
| User portal | `https://feam.613202690.xyz` |
| Admin pages/APIs | `https://admin.613202690.xyz` |
| Individual workspace | `https://u-<opaque-id>.613202690.xyz` |

These subdomains do not require separate registrations or another registrar nameserver update. They fit Cloudflare Universal SSL's first-level coverage in the full DNS setup when proxied; actual certificates remain to be verified. Keep the admin role checks and separate workspace origins from the design. These names are planned, not configured endpoints. Nested names such as `admin.feam.613202690.xyz` are outside the default certificate coverage and are not part of this plan. [Cloudflare certificate coverage](https://developers.cloudflare.com/ssl/edge-certificates/universal-ssl/limitations/)

Manage only explicit demo DNS records, Tunnel routes and Access applications in `613202690.xyz`; preserve the registration, zone, nameservers and unrelated resources during setup and teardown. Leave the owner's `saifshaikh.ca` zone untouched. The owner's explicit switch restores the `feam.613202690.xyz` / `admin.613202690.xyz` plan.

### Spaceship registration with Cloudflare DNS

Registration and nameserver setup are complete for the selected demo domain. On 2026-09-25, the owner confirmed Cloudflare Active status; queries through both `1.1.1.1` and `8.8.8.8` returned `mark.ns.cloudflare.com` and `norah.ns.cloudflare.com`. See the [activation record](ubuntu_web_dev_demo_workplan.md#domain-activation-and-hostname-switch-2026-09-25) for the scope of this evidence.

The owner first reported numeric `.xyz` names available at Spaceship for $0.95 (interpreted as 95 cents), then confirmed purchasing `613202690.xyz` there. Exact paid total and renewal terms have not been supplied; those are operator records to capture, not reasons to request another purchase.

Spaceship's public `.xyz` promotion lists US$0.75 with code `XYZ52`, plus the US$0.20 ICANN fee shown in its domain pricing, which totals US$0.95 before tax. Its terms limit the offer to new first-year registrations and exclude premium names. This matches the reported amount but does not establish whether the owner's numeric-name quote is that promotion or a separate numeric price class. The general `.xyz` table lists US$13.77 renewal plus US$0.20 ICANN fee; **do not apply that generic rate to this numeric name without checking, and do not assume its renewal is US$0.95**. No name-specific renewal quote was obtained. [Spaceship promotion and terms](https://www.spaceship.com/promos/), [domain pricing and ICANN fee](https://www.spaceship.com/domains/)

Keep the domain registered and renewed at Spaceship while Cloudflare provides DNS. Spaceship's custom nameserver setup is complete; no repeat nameserver change, registrar transfer or Spaceship hosting/email purchase is needed. The owner has connected Cloudflare to Codex; remaining edge work is verification of that connection's permissions and automation authentication, plus the demo's DNS routes, HTTPS, Tunnel and Access configuration. [Spaceship custom nameserver instructions](https://www.spaceship.com/knowledgebase/connect-domain-custom-nameservers/), [Cloudflare setup](https://developers.cloudflare.com/dns/zone-setups/full-setup/setup/)

### Lower-cost Cloudflare shortlist

**Owner correction, 2026-09-24:** Cloudflare displayed **$11.20/year for `613202690.xyz`**. The owner did not specify currency, taxes or separate initial/renewal amounts; preserve that quote as reported. The prior US$0.99 numeric-XYZ registry advertisement was not a verified Cloudflare quote and must not be used as this domain's Cloudflare budget. The reason for the price difference is unverified.

These are potential names to search in Cloudflare, with **indicative USD prices before tax** from the Cloudflare rows in public comparison tables, checked on 2026-09-24. They are not authenticated Cloudflare checkout quotes. Exact-name availability and standard versus premium pricing remain unverified.

| Candidate to search | Listed first year | Listed annual renewal | Price reference |
| --- | --- | --- | --- |
| `feam-demo.win` | US$4.18 | US$5.18 | [Cloudflare row for `.win`](https://tld-list.com/tld/win) |
| `feam-demo.date` | US$4.18 | US$5.18 | [Cloudflare row for `.date`](https://tld-list.com/tld/date) |
| `feam-demo.bid` | US$4.18 | US$5.18 | [Cloudflare row for `.bid`](https://tld-list.com/tld/bid) |
| `feam-demo.uk` | US$5.30 | US$5.30 | [Cloudflare row for `.uk`](https://tld-list.com/tld/uk) |

These are retained alternatives from before the Spaceship purchase; no additional domain is needed. `feam-demo.win` had the lowest listed price in this table, and `feam-demo.uk` was a similarly inexpensive alternative. For example, the first option would use `feam.feam-demo.win` and `admin.feam-demo.win`. These are candidates, not a claim of the absolute cheapest available name. Cloudflare's public domain-search page returned HTTP 403 to the automated read, so no live name-specific quote was obtained. Any future purchase would need a fresh exact-name, initial/renewal, currency and tax check in [Cloudflare's domain search](https://domains.cloudflare.com/).

Cloudflare's official [Registrar page](https://www.cloudflare.com/domains/) confirms its at-cost pricing model; that does not establish a specific name's price or guarantee identical registration and renewal amounts. A different registrar can also delegate DNS to Cloudflare without transferring the registration. [Cloudflare DNS setup](https://developers.cloudflare.com/dns/zone-setups/full-setup/setup/)

### Earlier alternatives and lookup evidence

The [XYZ registry](https://gen.xyz/number) still advertises 6–9 digit numeric names at US$0.99/year including subsequent years. Treat this only as an alternative to investigate with a registrar that actually quotes that class; the owner's Cloudflare quote takes precedence for `613202690.xyz`. Earlier Porkbun references were US$11.08 registration/renewal for standard `.com` and US$2.04 initially / US$14.21 renewal for ordinary `.xyz`; these are historical comparison references from the first research pass, not current checkout quotes. [Porkbun table](https://porkbun.com/products/domains)

Earlier read-only registry RDAP checks returned HTTP 404 for [feam-demo.com](https://rdap.verisign.com/com/v1/domain/feam-demo.com), [feam-demo.xyz](https://rdap.centralnic.com/xyz/domain/feam-demo.xyz), and [613202609.xyz](https://rdap.centralnic.com/xyz/domain/613202609.xyz). The last name has different digits from the owner's `613202690.xyz`; those results establish no availability for the requested name or the new Cloudflare candidates. A missing registration record does not guarantee checkout availability, an unreserved name, or non-premium pricing. Nothing was registered or placed in a cart during those checks; the owner subsequently purchased the selected name separately.

## Compute candidates

Preserve the current full-service sizing candidate: 8 x86-64 vCPUs, 16 GiB RAM, about 320 GiB service storage **plus** OS/scratch. CPU counts and shared-CPU prices do not prove ten-user capacity. Do not silently shrink quotas because the first dataset is small. GB/GiB units and remaining boot-disk space need checking.

| Candidate | Current public price reference | Fit and unresolved quote items |
| --- | --- | --- |
| **DigitalOcean Basic, New York or Toronto** | 8 shared vCPUs, 16 GiB RAM, 320 GiB boot disk: US$0.14286/hour, US$96/month cap | Strong first hourly candidate. A separate 320 GiB service volume is US$32/month, billed proportionally while it exists. This satisfies the planned service allocation without squeezing the OS into it. Confirm size availability and exact hourly volume rate. |
| **OVHcloud VPS-4, nearby full datacentre** | Canadian storefront advertises “from $32.21/month”, 8 vCores, 24 GB RAM, 200 GB disk | Potentially inexpensive for longer use, but the disk is insufficient alone. Exact local availability, additional disk, currency/tax, commitment, renewal/cancellation and container support need checkout verification. Do not apply an annual/monthly offer as an hourly rate. |
| **Hetzner CX43, Germany/Finland** | 8 x86 vCPUs, 16 GB RAM, 160 GB disk; US$0.0296/hour or US$18.49/month, excluding IPv4/tax | Lower bare-compute price; requires a service volume and transatlantic latency testing. The public product page reports unavailable stock in its rendered view; console availability is unverified. These EU prices do not apply to Hetzner US regions. |

Sources: [DigitalOcean plans](https://www.digitalocean.com/pricing/droplets), [regions](https://docs.digitalocean.com/platform/regional-availability/), [volume billing](https://docs.digitalocean.com/products/volumes/details/pricing/), [OVHcloud VPS offers](https://www.ovhcloud.com/en-ca/vps/), [OVHcloud Canada VPS location](https://www.ovhcloud.com/fr-ca/vps/vps-canada/), [Hetzner size/location details](https://www.hetzner.com/cloud/cost-optimized/), [current Hetzner price adjustment](https://docs.hetzner.com/general/infrastructure-and-availability/price-adjustment/).

For scale, 20 allocated hours on the listed DigitalOcean VM cost about **US$2.86 for compute**, before its volume, retained archive, inference, taxes and any extra transfer. The monthly compute-plus-320-GiB-volume references total US$128 if kept allocated for the applicable billing caps. These are arithmetic illustrations, not an approved all-in budget. A short demo does not require committing to the full month.

DigitalOcean and Hetzner continue billing allocated powered-off servers. Teardown must delete the disposable server; separately billed volumes/IPs need their own verified cleanup. Retained research storage deliberately stays. [DigitalOcean billing](https://docs.digitalocean.com/products/droplets/details/pricing/), [Hetzner billing](https://docs.hetzner.com/cloud/billing/faq/)

## Durable research storage

**Cloudflare R2 Standard is the first candidate**, since the owner already has Cloudflare. It is separate from disposable VM storage and can retain research from both hosts. Its published allowance includes 10 GB-month of storage, 1 million Class A and 10 million Class B operations monthly. Additional standard storage is US$0.015/GB-month; operations above the allowance cost extra. Direct R2 egress is listed free. [R2 pricing](https://developers.cloudflare.com/r2/pricing/)

For example, 20 GB-month of average retained data would have a US$0.15 storage charge after an otherwise unused 10 GB-month allowance, plus any chargeable operations/taxes. The allowance is shared with other use in the account, so this is not a promise of free storage. Use private access, scoped collector credentials, encrypted archival data, checksums and restricted research exports. Keep its Terraform state/lifecycle independent of demo compute; normal teardown must lack permission to delete the archive.

Research survival is confirmed. Retention duration, authorized reviewers, account enablement and any storage-spending ceiling remain to be configured. Shutdown drains and verifies archived traces/exports before source deletion; test retrieval after deleting a disposable VM. This is evidence preservation, not environment recovery.

The owner accepts intermittent pushes to Git and is open to a cheap, better-suited alternative. Proposed batching is every five minutes or 8 MiB, plus a final verified flush on shutdown. R2 remains the proposed default; implement a single selected archive destination. A separate **private Git repository** is viable for small text batches and curated exports, with a scoped credential and bounded files. Growing histories increase clone/storage overhead, and deleting an expired trace from the latest commit does not remove its history. GitHub blocks regular Git files larger than 100 MiB and recommends small repositories; history removal also affects existing clones. These constraints favor object storage for collected traces. [GitHub file/repository guidance](https://docs.github.com/en/repositories/working-with-files/managing-large-files/about-large-files-on-github), [history removal](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/removing-sensitive-data-from-a-repository)

## Before requesting a purchase or deployment approval

Use `feam.613202690.xyz` in the active `613202690.xyz` zone; start with the [existing Codex Cloudflare connection](ubuntu_web_dev_demo_workplan.md#cloudflare-connected-to-codex-2026-09-25), verify its account/permissions and automation authentication, and validate the selected demo routes, HTTPS and Access. Retain the purchased domain's actual renewal terms for operator records. Prepare the exact cloud plan for the selected account/region, including storage/IPs/tax, per-run cap, monthly residual resources and teardown behavior. Check available capacity and the intended 24.04 x86-64/runtime features. Validate Ottawa browser latency and ten-user capacity under a separately authorized finite test. Request additional scoped Cloudflare credentials only for a missing edge/archive/automation capability when needed, and the OpenRouter key only when the broker/live check needs it. The earlier research used no credentials, paid resources, domain purchase or inference.
