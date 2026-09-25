# Ubuntu web demo environment

Status: proposed design, including the IaC evaluation and deployment contract; deployment artifacts and services remain unimplemented.

Updated: 2026-09-24. Machine baseline inspected on 2026-09-23 at repository revision `eceddcc`; this revision records the owner's cohort, authentication, data, model-budget, and minutes-scale cloud-recovery decisions alongside the IaC contract. The IaC assessment uses that recorded baseline, the owner's confirmation of a 2 TB SSD, and upstream documentation, not a fresh inspection of the Ubuntu host.

Implementation sequence and progress: use the [development workplan](ubuntu_web_dev_demo_workplan.md) for checkable tasks, Mac versus Ubuntu execution boundaries, dependencies, and evidence gates. This design remains the architectural source of truth; task completion is recorded in the workplan.

## Recommendation

Use this Ubuntu machine to run a small, invite-only FEAM service: **Cloudflare Tunnel + Cloudflare Access email PIN login + one isolated container per demo user, with powerful hosted model assistance enabled by default**. Phase 1 demonstrates the strongest useful assistance supported by FEAM and records user workflows and outcomes to inform Phase 2 small language model (SLM) development. Admins manage users, sandboxes, model budgets, dataset jobs, and dataset access through a dashboard. A separate pipeline loads and validates datasets, publishes versioned FEAM provider releases, and makes approved releases available read-only to authorized sandboxes.

Keep infrastructure inexpensive while reserving model spending for the purpose of the demo. Cloudflare lists a **$0 Zero Trust plan for up to 50 users**, enough for 10 demo users and a small number of admins. The Ubuntu deployment needs no rented VM, managed database, email service, analytics service, or managed pipeline service; cloud fallback has separate costs. **Hosted inference is a separate, expected usage cost**, alongside domain, electricity, internet, and backup storage. The edge-plan baseline was checked on 2026-09-23; the domain/source comparisons were checked on 2026-09-24. Recheck external prices and limits before deployment. [Cloudflare pricing](https://www.cloudflare.com/plans/)

Make the installation reproducible with **Ansible for Ubuntu configuration, systemd for supervision and reboot recovery, and Terraform CLI for Cloudflare and cloud resources**. Build and retain approved artifacts outside demo sessions. The owner requires recovery within minutes at the same service capacity, but explicitly does **not** require workspace continuity between Ubuntu and cloud. The revised candidate is a **fast cold start from a prebuilt image, prepared datasets, and small durable control-state checkpoints, with fresh cloud workspaces**. A running standby is an alternative only if measured cold-start time is unacceptable. See [IaC evaluation and ownership](#iac-evaluation-and-ownership) and [Cloud portability and recovery](#cloud-portability-and-recovery).

Access email PIN login is accepted by the owner; clickable magic links are not required. A new domain is needed: a free GitHub Pages hostname is not a substitute for the controllable DNS zone used by this architecture. See [Domain and hostname choice](#domain-and-hostname-choice).

## Requirements and scope

Confirmed requirements:

- Use this Ubuntu machine as the web-accessible demo host.
- Support admins and regular users, targeting up to 10 users in FEAM sandboxes.
- Regular users use the FEAM CLI/TUI and available datasets. Admins only manage the demo environment; the admin role does not allocate a development workspace or editor.
- A dataset-loading pipeline prepares datasets on the demo host and makes them accessible to selected sandboxes.
- Phase 1 provides powerful hosted model assistance as the normal demo experience, captures user usage patterns, and produces evidence and reusable, eligible examples for Phase 2 SLM development.
- Prefer free and low-cost options. Email allowlisting with passwordless login is a suitable authentication direction.
- Reproduce the Ubuntu installation through IaC, recover automatically from routine service/host restarts, and make the same deployment usable as a low-cost cloud fallback.

### Confirmed cohort and deployment decisions

| Topic | Owner-confirmed decision |
| --- | --- |
| Initial accounts | Four admins and three regular users, all in Ottawa. Admins receive management access, not workspaces. Keep capacity for ten regular users. |
| Enrollment and login | Invitation only, exact-email allowlists, Cloudflare Access email PINs; no public enrollment or whole-domain grants. |
| Retention and access | The proposed workspace retention, role permissions, separate audiences, and dataset-grant model are accepted. |
| Initial data | Small Government of Canada open-data climate datasets; valid GeoTIFF `.tiff` rasters and Parquet-only published tables. |
| Hosted model | `deepseek/deepseek-v4.1-flash` through OpenRouter, using the evaluated `deepinfra/fp8` route with provider fallback disabled. |
| Project model allowance | US$100 total, following OpenRouter's default currency. Pause new requests until an admin increases the allowance; no automatic monthly reset. |
| Participation | Owner confirms consent has already been obtained. Do not add a participant-notice or repeat-consent onboarding gate for this cohort. |
| Domain | Prefer free if compatible; otherwise minimize registration plus renewal cost. No existing domain is available. Exact name/checkout remain pending. |
| Cloud service | Same service capacity and features as Ubuntu, with an accepted cold-start recovery target of at most 15 minutes after failure detection. A reduced-cohort fallback is not acceptable. Timing remains unverified until a deployment drill passes. |
| Cross-host workspaces | Workspace loss on switching hosts is acceptable; Ubuntu and cloud do not need shared or replicated private workspaces. Accounts, permissions, approved datasets, and project spending remain common service state. |

The exact seven email addresses are retained in the ignored local planning file `.local/demo-deployment/participants.yaml`, with owner-only permissions. This is not executable IaC or an encrypted backup. Move it into encrypted operator configuration before provisioning; do not commit identities into this public design, Terraform configuration/state, images, or research exports. Bootstrap all four admins and three users idempotently by exact identity, then let the control database and membership reconciler own changes. Repeated provisioning must not recreate removed accounts or overwrite later roles. No invitations or account creation have been performed.

Proposed operating defaults:

- Capacity target: 10 simultaneous lightweight demo sandboxes and a management dashboard, with at most one bounded dataset-loading job when headroom permits.
- Each regular user owns one persistent demo workspace containing synthetic practice fixtures and writable outputs. Pipeline-loaded datasets are stored once per release and shared read-only with authorized sandboxes.
- Regular users may run shell commands inside their container. Treat browser terminal access as code execution even when the launcher opens the TUI first.
- Idle workspaces stop after 30 minutes; data remains for 30 days after last use, with advance notice before deletion. Explicit reset requires confirmation.
- Hosted assistance is on by default. Sandboxes have no general internet access; model traffic passes through a private, authenticated inference broker with server-side budgets.
- Structured interaction capture is part of Phase 1. Record the owner's confirmation of existing consent in private deployment records, without a new participant notice or repeat-consent gate; keep account/billing records separate from the pseudonymous research corpus.
- One host is active at a time. Persistent files survive routine restarts on that host; a host reboot or failover requires a new terminal/TUI process. Switching hosts may create fresh private workspaces, as accepted by the owner. Minutes-scale service recovery is required, not uninterrupted terminal sessions. Targets below remain unverified until deployed acceptance; this does not establish HPC acceptance.

## Existing machine and FEAM implementation

Read-only inspection produced the following snapshot; it is not a load test.

| Item | Observed | Design implication |
| --- | --- | --- |
| OS | Ubuntu 24.04.5 LTS, x86-64 | Pin and test compatible container/runtime packages. |
| CPU | Ryzen 5 3600, 6 physical cores / 12 threads | Suitable candidate for interactive demos; bound dataset-processing concurrency. |
| RAM | About 16 GiB total; about 12 GiB available during inspection | Budget sandboxes and the dataset pipeline separately. |
| Swap | 4 GiB | Emergency margin; exclude it from capacity calculations. |
| Physical storage | Owner confirms a 2 TB SSD on 2026-09-24 | Ample nominal capacity for the initial allocation; map the device to mounted filesystems and measure current free space during preflight. |
| Root disk | 183 GiB filesystem, 136 GiB available | Keep bounded system logs and image caches. |
| Home disk | ext4, about 1.7 TiB total / 1.5 TiB available | Put FEAM service storage on this disk, in its own service directory. |
| Resource controls | cgroup v2 | Verify actual CPU/memory/PID delegation before admitting users. |
| Existing runtime | Docker and containerd services active; Docker executable present | A dedicated rootless runtime remains to be configured and tested. |
| Web tooling | No `cloudflared`, `ttyd`, Caddy, or nginx found on the inspected PATH; checked web service units inactive | Plan the tunnel, terminal, and portal as new deployment components. |

During the recorded Ubuntu inspection, the shell sandbox failed to start with a network-namespace permission error. Inspection therefore used approved read-only commands outside that sandbox. This is a deployment preflight concern, not proof that Docker containers cannot run. Test the intended runtime and namespace policy explicitly; do not globally disable host protections to make the service work.

The [workspace README](../feather-mesh/README.md), [peer-access contract](data_access_contract.md), and [TUI demo runbook](tui_agent_stage1_demo.md) establish these integration constraints:

- FEAM's current interactive interface is an optional Rust TUI; there is no existing multi-user web portal in the inspected implementation.
- The Cargo binary is `mesh_cli`; the installed user command should be `feam`.
- Peer operations require `--project ROOT`. Authoritative data publication is in each provider's `serving/manifest.json`; legacy SQLite is separate.
- The demo script already creates a provider and a `client with spaces`, including Parquet and GeoTIFF fixtures and an intentionally unavailable peer.
- STAC is an unauthenticated, exact-IPv4-loopback metadata service returning local file URIs. It is not an internet dataset-download service and must remain inside the isolated workspace container.
- Existing [footprint measurements](tui_agent_stage1_acceptance.md#footprint-and-conditions) are small, sampled, single-user macOS measurements. They do not prove Ubuntu peak memory or 10-user capacity.

## IaC evaluation and ownership

The original tool split is suitable after separating resource provisioning, host configuration, application state, and process recovery. IaC can recreate the environment; the controller and durable stores must recover interrupted application work. Rerunning an infrastructure apply is not the routine restart mechanism.

| Option | Assessment for this demo | Decision |
| --- | --- | --- |
| Ansible + systemd | Fits an existing Ubuntu installation: packages, service identities, fixed storage, unit files, and repeatable configuration. systemd can recover services without an operator laptop or provisioning runner. | Primary host implementation. Use the same roles for local and cloud inventories. |
| Terraform CLI | Fits API-managed DNS, tunnels, Access applications/policy structure, backup storage, and cloud VM/volume/firewall resources. Host package installation through shell provisioners would duplicate Ansible's job. | Use for external resources; no Terraform Docker provider for users' live containers and no host setup through `remote-exec`. |
| OpenTofu | An alternative with an MPL-2.0 open-source license. Its licensing/governance is a reason to choose it, but it does not save a Terraform CLI fee for this deployment. | Keep as an alternative, not a second supported engine. A future switch requires state/provider compatibility tests. |
| Docker Compose | Useful for a fixed local development stack. It does not replace host mounts, cgroup delegation, application recovery, or the grant-aware workspace controller. | Optional engineering convenience; systemd and the controller own the deployed lifecycle. |
| cloud-init / image baking | Useful for initial cloud access and the prebuilt image required by the accepted fast-cold-start candidate. It adds little to an already installed physical machine. | Use Ansible-backed image preparation and minimal cloud bootstrap for bounded startup recovery. Validate the prebuilt image in actual cold-start drills; do not compile the service during failover. |
| Kubernetes / a multi-host scheduler | Adds a control plane while persistent workspaces, authorization, and billing still need a recovery design. | Defer; ten terminals on one active host do not require it. |

**Terraform cost:** running Terraform CLI to manage this demo does not require a paid Terraform subscription. HCP Terraform and Terraform Enterprise are separate offerings; HCP is optional for this plan. Current Terraform uses the Business Source License, whose restrictions concern specified competitive offerings; OpenTofu uses MPL-2.0. Select Terraform here because no requirement calls for the alternative license. Cloud resources, storage, network usage, and hosted inference remain chargeable independently of the IaC engine. [HashiCorp licensing FAQ](https://www.hashicorp.com/en/license-faq), [Terraform editions](https://developer.hashicorp.com/terraform/intro/terraform-editions), [OpenTofu FAQ](https://opentofu.org/faq/)

Assign each changing resource one writer:

| Owner | Managed state | Boundary |
| --- | --- | --- |
| Terraform, run by the operator or scoped recovery runner | Dedicated DNS records, separate local/cloud tunnels, Access applications and static policy structure; cloud infrastructure and failover-controller identity | Import existing resources before managing them. Preserve application audiences and unrelated domain resources. Initialize route destinations, then explicitly ignore only the reconciler-owned active-target fields. No live workspace rows, email memberships, data bytes, or budget balances in configuration. |
| Failover controller, separately authorized | Active-site lease/generation, promotion journal, and active tunnel targets for the fixed demo hostnames | Single runtime writer of route targets; invoke only the approved cloud-recovery runner/plan, with no arbitrary infrastructure or repository-code execution. Terraform must not undo promotion on the next apply. Record route ownership/import/recovery semantics in the chosen provider implementation. |
| Ansible, run by the operator | FEAM service accounts, package versions, subordinate UID/GID assignments, storage pool and mount definitions, unit files, configuration, approved artifacts | Mutations stay within recorded FEAM resources. A normal converge cannot reset workspaces, restore databases, reformat existing storage, or replace the host's existing Docker service. |
| Application services | Accounts, mutable edge allowlists, grants, lifecycle jobs, container generations, dataset publication, consent, traces, and spending | Database state and application recovery rules govern; the gateway cannot run Ansible/Terraform or obtain operator credentials. |
| systemd | Service start/stop, restart backoff, mount ordering, timers | Boot uses installed configuration and pinned local artifacts. No source builds, migrations, dataset imports, or IaC applies on every boot. |

Resolve the Access-policy ownership conflict before implementing either writer. Terraform owns applications and policies that reference dedicated user/admin Access group IDs. The account reconciler owns those groups and their exact-email membership through the API; Terraform must not also declare the same group resources. Bootstrap creates the groups under the operator's control while public routing is disabled, records their IDs in private configuration, and then creates the referencing policies. Only the active deployment runs the reconciler. Represent zero members with a tested denying group configuration if the API rejects an empty include list; never broaden admission to make an empty group valid. Verify that removing a user followed by a Terraform apply does not restore access. Cloudflare supports reusable groups and programmatic policies; validate the selected provider schema and account permissions in the implementation. [Access policies](https://developers.cloudflare.com/cloudflare-one/access-controls/policies/), [Cloudflare Access group schema](https://registry.terraform.io/providers/cloudflare/cloudflare/latest/docs/resources/zero_trust_access_group)

Keep Terraform state outside the demo host, with separate state for edge configuration, recovery storage, and replaceable cloud compute. Use an encrypted, versioned backend with a tested locking mechanism and recoverable operator credentials. Document how to bootstrap/recover the backend independently of the infrastructure it stores. Restrict deletes of backup storage and retained volumes through provider permissions as well as plan review. Marking a value `sensitive` suppresses output but does not itself encrypt state or saved plans. Exclude state, plans, real inventories, and secrets from the public repository and ordinary CI artifacts. [Terraform sensitive data](https://developer.hashicorp.com/terraform/language/manage-sensitive-data)

Use a private encrypted Ansible inventory/vault for host addresses, identity mappings, deployment IDs, and secret material; keep the decryption credential separately recoverable off-host. Install runtime secrets only for the service that consumes them, suppress secret task logs/diffs, and issue separate credentials for the pipeline, broker, tunnel, backup worker, and policy reconciler. Provisioning credentials stay with the operator. Exact tools, providers, collections, packages, host binaries, and image digests are recorded in a release manifest and lockfiles; updates are reviewed releases.

## IaC on the existing Ubuntu machine

**Assessment: a good fit for in-place Ansible provisioning, conditional on a fresh preflight and runtime proof.** The owner-confirmed 2 TB SSD and recorded 16 GiB RAM, cgroup v2, and large ext4 home filesystem support the proposed layout. No reinstall, repartition, hypervisor, or new VM is needed for the local host. The recorded 136 GiB of free root storage cannot hold the full proposed service allocation; use the larger filesystem after verifying its mapping to the SSD. Nominal SSD capacity is not the same as currently available filesystem capacity. The design-editing session is on macOS and has not verified Ubuntu connectivity, sudo, mount identities, or runtime enforcement.

| Recorded condition | Ubuntu provisioning decision | Evidence required before deployment |
| --- | --- | --- |
| Ubuntu 24.04.x, x86-64 | Target this OS family and `linux/amd64` artifacts; read actual release/kernel/package facts before selecting versions. | Python/Ansible compatibility, package origins, package conflicts, kernel, systemd, and rootless prerequisites recorded. |
| Existing Docker/containerd services | Create a dedicated `feam-runner` rootless daemon, private socket, and data root. Inspect current packages before adding compatible rootless tools. | Existing containers/services remain unaffected; controller inspection proves it addresses only the dedicated daemon. |
| Restricted user namespaces | Use supported RootlessKit packaging and its applicable host AppArmor policy. | An actual container starts under the dedicated identity; no host-wide namespace/AppArmor relaxation. |
| cgroup v2 | Enable lingering for the runner's systemd user manager and delegate required controllers to that identity. | Boot without interactive login works; measured memory/CPU/PID limits and aggregate ceilings hold. |
| 2 TB SSD; recorded large ext4 home filesystem | Record the filesystem UUID and mountpoint; create only the dedicated service directory and fixed backing images there. Put Docker's data root there too. | Available bytes/inodes, filesystem identity, loop-device support, mount behavior, ownership/ACLs, and non-overlapping subordinate ID ranges pass. |
| 16 GiB RAM, shared physical host | Retain the documented admission budgets; build artifacts elsewhere and run backups outside busy demo periods. | Measure existing load plus the actual cohort; a historical free-memory sample is insufficient. |
| Physical host and home internet | Supervise services at boot; configure advertised uptime with the machine operator. | Reboot/login independence, sleep behavior, external reachability, and power-return behavior are tested; firmware or network recovery may need manual setup. |

Docker documents that rootless resource controls require cgroup v2 **and** systemd; accepted CLI flags alone do not prove limits. Install the daemon as a systemd **user** service with lingering, rather than a system service carrying `User=feam-runner`. Constrain the runner's user slice and verify container processes land beneath it; a separate slice for gateway/broker/collector/pipeline does not automatically constrain rootless containers. Ubuntu's RootlessKit AppArmor allowance concerns creating user namespaces; it must not be described as proof of per-container AppArmor confinement, which rootless Docker lists as unsupported. Retain supported seccomp and other sandbox controls and measure the resulting boundary. [Rootless Docker operation and cgroups](https://docs.docker.com/engine/security/rootless/tips/), [Ubuntu prerequisites and rootless limitations](https://docs.docker.com/engine/security/rootless/troubleshoot/)

The read-only preflight should collect `/etc/os-release`, kernel/systemd versions, filesystem UUIDs/mounts and free bytes/inodes, current Docker services/package sources, service-account/subordinate-ID collisions, cgroup controllers, AppArmor/user-namespace state, time synchronization, operator access, and required egress. Useful existing commands on Ubuntu include `uname -r`, `systemd --version`, `lsblk -f`, `findmnt -T /home`, `df -hT / /home`, and `df -i / /home`. Record evidence without dumping credentials or personal files. The first runtime test is a separate, bounded provisioning step.

### Storage provisioning contract

Parameterize the host storage root; `/home/feam-service-data` is the local proposal. Keep paths **inside** containers unchanged when moving to a cloud block volume. Fixed recorded identities and subordinate ID mappings make owned files restorable; where a replacement host has a collision, use an explicit ownership migration for restored service files. Never recursively change ownership of the personal home or an existing Docker data root.

Provision a fixed pool of ten 2 GiB workspace files and two 2 GiB reset spares on the dedicated service path. Track each pool slot, backing-file identity, filesystem UUID, and assigned generation. Format only a newly allocated, unassigned file whose ownership is proven; an existing or unexpected filesystem stops provisioning. The operator mounts this finite pool; the controller only assigns/recycles recorded slots after stopped generations meet retention rules. Normal deployment preserves assignments and bytes. If retained generations consume both spares, reset queues until a slot is eligible; it never creates unbounded extra volumes.

Use a proposed 200 GiB dataset filesystem for the 50 GiB staging and 100 GiB retained-release budgets, including headroom for filesystem overhead and admission thresholds. Those logical limits remain subject to actual free-space and complete-candidate reservation checks. The previous 150 GiB filesystem proposal left no margin at the configured maxima. Ten workspaces plus two spares reserve 24 GiB; adding the dataset filesystem, 5 GiB operations, 20 GiB traces, and a proposed 20 GiB image/runtime allowance gives approximately **269 GiB before additional backup scratch and underlying-filesystem reserve**. Final physical allocations must also cover metadata and the chosen enforcement method. This fits comfortably within the stated 2 TB SSD and recorded home free space, subject to preflight; do not reserve seven uncompressed local copies of the dataset pool.

Mount staging and releases on the same dataset filesystem so promotion remains an atomic rename. System mount units identify storage explicitly and order it before consumers; verify required paths are the expected mounted filesystems, not just existing directories. Order the runner's system user-manager instance after its required host storage, then start its rootless user service. Cross-manager dependencies need a tested bridge: a user unit cannot simply require a system mount unit. Mount loss closes admission and stops affected workloads without writing into underlying empty mountpoint directories. Boot-time socket tmpfs mounts and ACLs are recreated from the finite workspace inventory.

### Proposed repository artifacts and operator workflow

The following paths are deliverables to implement, **not existing deployment entrypoints**:

```text
infra/demo/
  README.md                         operator workflow, recovery, and evidence
  terraform/
    edge/                           DNS, tunnels, Access applications/policies
    recovery-storage/               encrypted backup/state storage, separate lifecycle
    cloud-host/                     on-demand full-capacity VM, data volume, firewall
  ansible/
    inventories/                    sanitized local/cloud inventory examples
    roles/                          preflight, accounts, runtime, storage, services, backup, recovery
    preflight.yml                   read-only host assessment
    host.yml                        converge recorded host resources
    deploy.yml                      install approved release, migrate, health-check
    restore.yml                     explicit recovery into stopped/new service storage
  images/                           FEAM terminal and prebuilt cloud image manifests
  tests/                            provision/reboot, cold start, failover and handback checks
```

The control machine is the operator's workstation or a trusted runner with the same locked toolchain. Ansible reaches Ubuntu over verified SSH on a private/limited management path, or through a local operator invocation on Ubuntu. Public demo routing is not the sole recovery connection. Separate preflight, host converge, release deployment, and restore so repeating setup cannot accidentally replay a data restore. Bootstrap the account/database only when intentionally creating a new installation; missing state in an established installation is a recovery error.

Before exposure: review the host preflight, converge service accounts/storage/runtime, install approved artifacts and private configuration, initialize local control state/groups, apply the Terraform edge plan, validate identity and application readiness, and enable public routing last. Plan/apply jobs run outside the user-facing service. Import existing edge resources and retain the same audience IDs on subsequent applies. Protect cloud data independently of compute replacement. Cloud provisioning requires an approved account and infrastructure budget; an off-host recovery runner must be able to execute the locked cloud plan without the Ubuntu host or an operator laptop.

Implementation CI must add Terraform format/validation and provider-lock checks, Ansible syntax/lint checks, secret scanning, and unit-file validation when the artifacts arrive. Test a disposable Ubuntu VM with real systemd/cgroups/loop mounts; container-only CI does not establish those behaviors. Require an initial converge, a second converge with no unintended changes, reboot without a login, and restore into a fresh host. Ansible check mode is only a preview: unsupported tasks can be skipped and tasks can override it. Restrict overrides to audited read-only probes and use `no_log`/disabled diffs for secret-bearing tasks. [Ansible check/diff mode](https://docs.ansible.com/projects/ansible/latest/playbook_guide/playbooks_checkmode.html)

## Architecture

```mermaid
flowchart LR
    B[User browser] --> E[Cloudflare HTTPS and Access]
    E --> T[Outbound Cloudflare Tunnel]
    subgraph H[Ubuntu machine]
        T --> G[Portal and authorization gateway]
        G --> D[(Local control SQLite)]
        G --> C[Workspace controller]
        C --> R[Dedicated rootless Docker runtime]
        R --> U[Per-user FEAM demo containers]
        G -->|Private Unix sockets| U
        U --> V[Per-user persistent data]
        G --> J[Dataset job queue]
        J --> P[Fetch, validate and publish pipeline]
        P --> S[Private staging]
        S --> Q[Approved provider releases]
        C -->|Grants and release selection| Q
        Q -->|Read-only mounts| U
        U -->|Private inference socket| M[Inference broker and budget ledger]
        U -->|Structured interaction events| I[Usage collector]
        M -->|Usage and response events| I
        I --> F[Private trace store and curated SLM exports]
    end
    X[Approved dataset sources] --> P
    M --> L[Powerful hosted model]
```

Cloudflare Tunnel connects outward from the machine and does not require a public IP or router port forwarding. Cloudflare supports proxied WebSockets, which the terminal needs. Test reconnects through the complete deployed path. [Tunnel documentation](https://developers.cloudflare.com/tunnel/), [WebSocket documentation](https://developers.cloudflare.com/network/websockets/)

| Component | Responsibility |
| --- | --- |
| Cloudflare DNS, Tunnel, Access | Public HTTPS, email verification, coarse access policy, routing to the private origin. |
| Portal/gateway, new component | Validate identity, enforce roles and ownership, present workspace controls, proxy authenticated HTTP/WebSockets. A small Go service with server-rendered pages is a proposed implementation; no SPA is required. |
| Local control SQLite | Accounts, roles, workspace assignments, dataset jobs, release assignments, grants, revocation state, audit events. It is separate from FEAM's legacy registry and is not a dataset publication authority. |
| Workspace controller, new component | Reconcile lifecycle jobs, enforce grants when selecting dataset mounts, and invoke only approved runtime templates. The gateway never gets the Docker socket. |
| Rootless Docker under a dedicated service account | Run demo containers without using the personal account's home or existing daemon as the control plane. |
| `ttyd` + FEAM | Provide an interactive terminal inside each demo container. |
| Dataset pipeline, new component | Run approved fetch/validation jobs, publish through FEAM, prepare complete releases, and report results to admins. Separate service credentials and private staging from sandboxes. |
| Shared provider storage | Hold versioned dataset releases and their authoritative `serving/manifest.json` files; authorized sandboxes receive read-only mounts. |
| Inference broker, new component | Keep upstream API keys outside sandboxes; enforce the selected model/provider, disclosure policy, active-user checks, shared budget reservations, concurrency, and usage accounting. |
| Usage collector and private trace store, new components | Correlate sanitized interactions with tools, reviews, actual outcomes, model usage, and feedback; prepare reviewed exports for Phase 2. |
| systemd, journald, timers | Supervision, bounded logs, idle stopping, backups, and cleanup; units/mounts/configuration installed by Ansible. |

Use example hostnames such as `feam.example.org` for the user portal, `admin.example.org` for all management pages/APIs, and `u-<opaque-id>.example.org` for a demo workspace. These are placeholders, not configured domains. Keep workspace content on separate origins from the portal; never proxy arbitrary user content under the portal's origin.

Configure explicit hostname routes for the initial small user pool, with an unmatched-host deny rule. All routes terminate at the authorization gateway, which listens only on loopback or a private Unix socket. Every workspace hostname maps to one server-side workspace record. A hostname or unguessable ID is not an access credential. Cache bypass applies to authenticated pages, terminal traffic, and authentication responses.

The controller accepts workspace IDs and fixed actions, not arbitrary host paths, image names, command strings, or mount specifications. Lifecycle requests use structured arguments and idempotency keys. Separate Unix socket permissions restrict controller access to the gateway. Neither service runs as host root; initial OS/runtime/storage provisioning belongs to the machine operator.

### Domain and hostname choice

GitHub Pages provides free `<owner>.github.io` static hosting, not a DNS zone the project can delegate to Cloudflare or point at its Ubuntu/cloud tunnel. It could host a separate public landing page linking to the demo, but does not remove the protected backend's hostname requirement. The selected browser-only Tunnel/Access flow publishes applications under a domain on Cloudflare. [GitHub Pages](https://docs.github.com/en/pages/getting-started-with-github-pages/what-is-github-pages), [Cloudflare published applications](https://developers.cloudflare.com/learning-paths/clientless-access/connect-private-applications/create-tunnel/)

Register one inexpensive domain with configurable authoritative nameservers; use Cloudflare DNS with single-level `feam`, `admin`, and `u-<opaque-id>` hostnames. The registrar need not be Cloudflare. Do not buy separate domains, email hosting, or web hosting for each role/user. Retain the same names and Access audiences across failover. A Quick Tunnel's random `trycloudflare.com` URL is for development/testing, not this stable invited service. [Quick Tunnel limitations](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/do-more-with-tunnels/trycloudflare/)

For the owner's lowest-cost preference, first price an available **6–9 digit numeric `.xyz` domain**: the registry advertises its special 1.111B class at US$0.99/year including renewal. This is a candidate, not a verified registrar checkout or proof of the absolute cheapest offer. Ordinary word-based `.xyz` names are a different tier: Porkbun currently lists US$2.04 initially and US$14.21 renewal. Verify the exact name, registrar fee, renewal, tax/currency, nameserver support, and Ottawa participants' institutional browser access before purchase. Avoid first-year-only discounts or bundled hosting as claims of a permanently free domain. No domain has been purchased. [Numeric-class offer](https://gen.xyz/1111b), [Ordinary `.xyz` pricing](https://porkbun.com/tld/xyz)

## Roles and user experience

| Capability | Regular user | Admin | Machine operator |
| --- | --- | --- | --- |
| Start, stop, reset own demo | Yes | Manages users' sandboxes; none allocated by the admin role | Recovery access |
| Use FEAM CLI/TUI; edit data in own sandbox | Yes | Management dashboard only | Recovery access |
| Read another user's workspace | No | No by default; explicit audited support action only | Technically possible on the host |
| List users, assign role, disable access | No | Yes | Bootstrap/recovery |
| Stop/reset another workspace | No | Yes, with confirmation for reset | Recovery access |
| Set quotas within host policy; view utilization and job status | Own usage only | Yes | Set host-wide ceilings |
| Run approved dataset-loading jobs and approve releases | No | Yes | Configure pipeline templates and source credentials |
| Assign/revoke dataset bundles for sandboxes | No | Yes | Recovery access |
| Use hosted assistance within assigned budget | Yes | Configure the demo model/profile and spending limits | Provision upstream credentials |
| View aggregate usage; curate/export consented traces | Own session status | Yes, with a separate research-export permission | Configure storage and recovery |
| Change demo release | No | Select an already tested, approved image | Prepare/approve releases |
| Host sudo, arbitrary mounts, Docker socket, personal SSH keys | No | No through the website | Local administration only |

The machine operator is an operational responsibility, not a third public signup role. Initially it can be the owner of this machine. Website admin status does not automatically grant host administration.

Regular-user flow: open the site, authenticate by email PIN, then select **Open assisted FEAM**. Existing consent is recorded administratively; no participant notice or repeat-consent dialog blocks entry. Start the TUI with hosted assistance already enabled and explain how to ask for help. Show the selected model, assigned datasets, usage/budget status, recording status, and reset behavior as ordinary service status. Users ask the assistant to discover and explain data, resolve a pinned version, stage a private copy, and prepare publication/withdrawal actions for local review. Keep the shell and manual controls available for exploration or recovery. Manual fallback during an outage is clearly labeled and is not counted as a successful hosted demo.

Admin flow: use the management dashboard to approve users, set sandbox quotas and model budgets, stop/reset sessions, select approved FEAM images and model profiles, and inspect health, task outcomes, and audit history. A **Datasets** page manages loading jobs, validation, release approval, assignment, and withdrawal. A **Usage** page shows adoption, common tasks, friction, completion, latency, and cost, with controlled trace review/export for Phase 2. Routine management uses these controls and does not require a shell, source checkout, or editor.

## Authentication

### Recommended: allowlisted email PINs

Cloudflare Access emails a one-time code only when the email satisfies its Access policy; codes are single-use and expire after 10 minutes. The login page returns a generic message for disallowed addresses. This avoids operating an SMTP service or implementing token issuance. [Cloudflare email PIN documentation](https://developers.cloudflare.com/cloudflare-one/integrations/identity-providers/one-time-pin/)

1. Bootstrap the four supplied admins locally from private configuration; provide no public signup or self-promotion endpoint. Initialize the three supplied regular users through the same exact-email reconciliation path.
2. An admin adds an exact email address and role in the control database. Assign an immutable local user ID; do not use email strings as directory names.
3. A reconciliation task updates the corresponding dedicated Cloudflare Access group's exact-email membership using a narrowly scoped API credential provisioned by the operator. Terraform manages the referencing applications/policies, not these mutable groups, as specified in [IaC evaluation and ownership](#iac-evaluation-and-ownership). New accounts remain `pending` until the edge membership and any regular-user workspace assignment are ready; admin accounts require no workspace. Admins see synchronization status in the dashboard. An operator command provides recovery if automatic reconciliation fails.
4. Users enter their email at Access, receive a PIN if allowed, and submit it to authenticate.
5. The gateway validates the Access JWT's signature, issuer, intended application audience, and time bounds using a maintained JWT library and the documented key endpoint. It then looks up the active local account and checks the requested role/workspace. Plain email headers are never authentication. [Access JWT validation](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/authorization-cookie/validating-json/)
6. Use a separate Access application/audience for the admin host, with an exact admin email list and a proposed one-hour session. User portal/demo sessions may last eight hours. Require the audience matching the requested host; a regular-user token cannot authorize an admin route or enqueue a dataset-loading job.

The control database is authoritative for application authorization; Access is an additional gate. Keep policy changes versioned in private configuration, report synchronization failures, and never broaden policy to all email users or an entire domain to resolve a mismatch. Normalize email consistently with the identity provider; do not collapse dots or plus-address aliases. Bind the verified identity to the local record and handle email changes administratively.

At the tunnel, require Access JWT validation for protected hostnames as well as gateway validation. Restrict expected hostnames and proxy headers; unknown routes fail closed. Tunnel connectivity alone is not authentication. [Tunnel origin Access settings](https://developers.cloudflare.com/tunnel/reference/origin-parameters/)

Maintain local authorization on every request and WebSocket upgrade. Track open streams by local account and workspace; periodically recheck eligibility and token expiry. Disablement increments an account authorization version, rejects new traffic immediately, closes active streams within 30 seconds, and stops its containers. Edge policy/token revocation follows even if a provider API call initially fails. Removing a user from a list must not leave an already-open terminal usable.

Use host-only secure cookies for any local gateway session, with `HttpOnly`, `SameSite`, and the `__Host-` prefix; do not issue shared parent-domain portal cookies. Strip Access assertions, identity headers, and gateway cookies before proxying into user-controlled containers. The gateway consumes authentication material; sandbox processes do not need it. Enforce exact Origin checks on WebSockets and CSRF protection on lifecycle/admin actions, including sibling-subdomain requests. Administrative state changes use POST and require a fresh authorized session.

Require MFA on the Cloudflare account that controls DNS and Access. Email-only admin login is the accepted invited-demo tradeoff; an MFA-enabled identity provider can replace it without changing workspace ownership. If stronger admin authentication is later required, make provider-enforced MFA a launch condition rather than assuming mailbox verification supplies a second factor.


## Sandbox boundary and FEAM integration

Use one container per demo user, including that user's private practice provider and client projects. Separate containers have separate writable filesystems, process namespaces, terminal sessions, and caches. Pipeline-loaded providers are shared through read-only mounts selected by the controller's grant checks. This models isolated FEAM workflows with shared inputs; it does not pretend to be a multi-node shared HPC filesystem.

The runtime should be rootless under a dedicated `feam-runner` account, with a non-root process inside each container. Use a read-only image, drop Linux capabilities, set `no-new-privileges`, retain supported seccomp protection, and impose resource limits. Apply the host RootlessKit AppArmor prerequisites described in [IaC on the existing Ubuntu machine](#iac-on-the-existing-ubuntu-machine); do not claim per-container AppArmor confinement for rootless Docker. Do not mount the personal home, host root, devices, SSH agents, runtime sockets, or controller secrets. Rootless containers reduce host privilege but share the host kernel: this boundary is intended for the invited limited-trust cohort using synthetic practice fixtures and approved public climate data, not unrestricted hostile-code hosting. [Docker rootless mode](https://docs.docker.com/engine/security/rootless/)

Regular demo containers use `--network none`. `ttyd` can listen on a Unix domain socket; mount only that workspace's socket directory so the gateway can reach it without giving the sandbox network access. Enable writable terminal input and origin checks, disable URL-supplied command arguments, and allow at most two browser terminal connections per workspace. The server chooses the fixed FEAM launcher or shell command. [ttyd options](https://github.com/tsl0922/ttyd)

Use restrictive per-workspace socket directories and explicit UID/GID mappings or ACLs. Back each writable socket directory with a small bounded filesystem, such as an operator-provisioned 1 MiB tmpfs with an inode limit, so it cannot bypass workspace storage limits. Prove that the gateway can connect and another sandbox cannot; do not solve rootless permission problems with world-writable sockets. Put no controller or Docker socket in these directories. Treat upstream responses as untrusted: filter authentication-related response cookies/headers and bound response sizes/timeouts.

An example container layout is:

```text
/opt/feam/                       immutable release, SDK, demo script and fixtures
/workspace/demo/provider/       this user's provider and serving manifest
/workspace/demo/client with spaces/
/datasets/demo-climate/          assigned shared provider serving root, read-only
/workspace/cache/               this user's FEAM_CACHE_DIR
/workspace/exports/             explicitly exported user results
/tmp/                          size-limited temporary filesystem
/run/feam-terminal/             this workspace's terminal socket only
/run/feam-model/                this workspace's private inference socket only
/run/feam-events/               this workspace's private event-ingestion socket
```

Build the release once outside demo sessions:

```bash
# Existing repository command; run from feather-mesh/ in a controlled build.
cargo build --release -p mesh_cli --features agent-hosted
```

Install the resulting `target/release/mesh_cli` as `/usr/local/bin/feam` in the image. Generate the existing binary fixtures during image build, install the supported Python SDK and pinned example dependencies, and package the demo script with the fixture tree it expects. Configure `FEAM_EXECUTABLE` and `FEAM_WORKSPACE` to point at that packaged layout. Users do not install packages or compile Rust to start a demo.

On first start, run the existing demo generator **inside the container** against `/workspace/demo`, then launch:

```bash
feam tui --project '/workspace/demo/client with spaces' \
  --agent hosted --agent-profile phase1-demo
```

Generate the practice demo at its final container path because the script writes a provider symlink. Copy its small source fixtures into each user's workspace; never share a writable provider manifest between users. The provisioning wrapper then adds the granted pipeline-provider links and peer configuration described below. This wrapper is new work; the existing demo generator does not attach external datasets. Preserve FEAM's project paths, manifest authority, review confirmations, and recovery journal semantics.

The private demo volume is disposable; reset stops the container, closes its sessions, creates a clean replacement generation, initializes it, reattaches currently granted datasets, verifies health, and switches the workspace record. Shared dataset releases remain intact. A failed initialization leaves the old stopped volume recoverable, but recovery must still enforce current grants. The controller operates on its own recorded volume IDs, never on a browser-supplied path or a user-controlled symlink. Garbage-collect old generations under a bounded retention rule.

If users try STAC, run its service and Python client in the same isolated container so loopback and local file URIs have the correct meaning. It has no application token and rejects non-`127.0.0.1` binds. Do not publish STAC directly to the internet as a substitute for the terminal. A future web dataset viewer or download API is a separate design.

Provision the `phase1-demo` profile outside the writable project, using the broker connection described below. The image includes the hosted feature and the launch profile; users do not need to supply an API key or manually turn assistance on. Existing TUI confirmations and core access rules still govern every operation.

## Dataset-loading pipeline and shared access

Run the pipeline as a local supervised worker with a SQLite job queue and optional systemd schedule. An admin selects a registered source, dataset definition, and audience, then starts or schedules a job from the dashboard. This needs no managed ETL platform or admin development workspace. A trusted CI system can later submit the same job specification through a scoped service identity; browser-user credentials do not become pipeline credentials.

The data definition records a source identifier and pinned object/version or checksum, exact asset inventory, FEAM product/version identity, required reuse metadata, expected size, and intended audience. It also records whether metadata may be disclosed to the hosted model and whether resulting traces are eligible for research/export. Access to a dataset does not itself grant permission to send its contents to a model or use it for training. Source credentials remain in the pipeline's service configuration.

Use a **dataset bundle** as the initial filesystem access unit: one provider namespace and serving root containing products approved for the same audience. Mount only granted bundles. Users with a shell can read every file in a mounted bundle directly, so filtering the FEAM catalog is not an access-control boundary. Split differently restricted datasets into separate bundles/namespaces. Never mount the parent directory containing every bundle or any private staging directory.

### Initial Canadian climate bundle

Use `demo-climate` for an initial bundle available read-only to all invited regular users. The owner accepts the proposed audience and permission model: admins approve exact releases, public descriptive metadata may be disclosed to the selected model, and eligible sanitized interactions may enter the existing-consent research workflow. Raw source or dataset payloads are not automatically uploaded to the model. More restricted future datasets require separate bundles and explicit policies.

These are verified source-catalog candidates, **not downloaded, size-measured, or FEAM-validated releases**:

| Source | Bounded first import | Published format and validation |
| --- | --- | --- |
| [AAFC 30-year Average Maximum Temperature](https://open.canada.ca/data/en/dataset/b3fda923-a875-4dde-bb67-873f9684a031) | One monthly 1991–2020 climate-normal raster, preferably an eastern Ontario/Ottawa-area subset; the catalog offers prepackaged 10 km GeoTIFF grids. | A georeferenced `.tiff` with explicit CRS, nodata, temperature units and climatology interval; verify a known bounded window. |
| [ECCC Daily Climate Observations](https://open.canada.ca/data/en/dataset/5f963c2d-d4ed-5a79-8a31-c9c582ca5098) | One verified Ottawa-area station and one complete historical year of temperature/precipitation observations. Pin station identity and actual available date range during import. | Parquet only, with typed dates, station identifier, units, null/missing-value and quality-flag handling; compare row counts and known values to the source. |

Propose a 250 MiB combined published seed-bundle ceiling, with separate bounded source-download/expanded-size limits; this is an admission limit, not a measured source size. Select a smaller source/subset if necessary. Pin the exact resource URL, retrieval time, upstream version where available, source/output SHA-256, license/attribution, and importer version. Preserve immutable prepared releases off-host so failover does not depend on downloading or processing a live government source.

ECCC's catalog supplies CSV/API data, not a ready-made Parquet release. A tested pipeline importer may fetch that source into private staging and convert it; CSV/JSON are never accepted as published tabular assets or mounted source files. Likewise, a `.tiff` suffix alone is insufficient: FEAM must validate actual GeoTIFF content and metadata. The current [STAC contract](data_access_contract.md#python-and-stac-adapters) limits native geometry to WGS84; inspect the AAFC raster and, if necessary, produce a tested EPSG:4326 derivative in the importer with recorded transform/resampling/nodata provenance, not a relabeled CRS. Keep original immutable source bytes privately for reproducibility. Importer implementation and real SDK/Rasterio checks remain delivery work.

Pipeline stages:

1. **Fetch into private staging.** Download from a registered source or import an operator-provided local artifact. Pin source identity and verify expected hashes; enforce byte, time, file-count, and expanded-size limits. Fetch workers may reach only approved upstreams; validate redirects and resolved addresses, and block host/LAN/metadata destinations. Dataset definitions are data, not executable scripts. Do not execute source-provided commands or unsafe archive entries.
2. **Prepare a candidate provider release.** For a new bundle, initialize a FEAM provider project with its unique namespace. For an update, copy the previous release into a private candidate, preserving its manifest revision, product/version records, and tombstones. Add new assets under new version paths. The initial implementation uses ordinary copies rather than writable hard links to released bytes. Candidate construction and quota checks account for the complete retained bundle, not only the incoming files.
3. **Validate and register through FEAM.** Call existing `validate-metadata`/`serve` workflows against the explicit candidate project root with complete metadata. Rust services enforce Parquet or GeoTIFF inspection, declared inventory, namespace, path, and version rules. Unsupported source formats require a separately tested importer to prepare supported outputs and provenance. Never publish by placing files in a directory, editing SQLite, or hand-writing a manifest. See the [peer-access contract](data_access_contract.md).
4. **Check the candidate as a consumer.** Resolve the exact registered inventory through a fresh client peer link; exercise known-value table queries or bounded raster windows and verify integrity as appropriate. The mounted serving tree must contain only bytes intended for that bundle's audience and required FEAM metadata/projections. Unregistered drafts, input credentials, and validation scratch stay outside it. Present the report, release hash, metadata, provenance, and size to the admin.
5. **Approve and promote a complete release.** Record approval for the exact candidate hash, make the payload immutable to sandbox/runtime accounts, and atomically rename it into a unique release directory on the same filesystem. Only then record it as an assignable release. A crash after the rename leaves a private orphan for reconciliation, not a partial visible release; a failed validation or publication leaves current assignments unchanged. Serialize updates per bundle and recheck the parent release before promotion to prevent concurrent jobs from losing each other's versions or withdrawals.
6. **Assign and attach.** Admins grant a bundle/release to all demo users or selected sandboxes. The controller verifies the current grant, namespace, release integrity/status, and canonical recorded source path, then attaches that release's `serving/` directory read-only. Newly started or recreated containers receive the selected release and peer configuration. Existing sessions remain pinned until the announced update/restart, except for access revocation below.

FEAM's atomic manifest write is the publication boundary inside each candidate; the pipeline's release promotion controls which complete provider snapshot is exposed to sandboxes. These are separate commits. Record job IDs and candidate hashes so a retry reconciles an uncertain commit instead of blindly repeating `serve` and creating a duplicate-version conflict. A failed derived projection must not be reported as an uncommitted manifest write. The control DB tracks deployment and grants; the mounted provider manifest remains FEAM's authority for registered products.

Example host layout and client mapping, all proposed:

```text
/home/feam-service-data/datasets/
  staging/<job-id>/provider/                 private candidate, never mounted
  releases/<bundle-id>/<release-id>/provider/
    .feam/project.toml                      pipeline/provider configuration
    serving/manifest.json                   authoritative registered inventory
    serving/datasets/<product>/<version>/   immutable released data

Sandbox:
  /datasets/demo-climate/                    read-only mount of assigned serving/
  /workspace/demo/client with spaces/peers/demo-climate
    -> /datasets/demo-climate
```

The provisioning wrapper appends the following peer to the existing client configuration, preserving its practice-demo peers:

```toml
[[peers]]
alias = "demo-climate"
namespace = "demo-climate"
path = "peers/demo-climate"
```

There must be only one attached release per provider namespace in a client. Use a namespace distinct from the practice fixture's `climate`. Run `feam refresh` for the explicit client project after attachment. Users read shared assets directly through the resolver/SDK; `consume` remains an explicit copy into their private quota. Transformations and outputs belong in that private space. Ten readers of one release share one stored payload rather than requiring ten full dataset copies.

Use explicit read-only bind mounts, private mount propagation, and no nested writable submounts. Validate read-only behavior with actual shell writes, including attempts through symlinks. A read-only mount does not stop its host owner from modifying the source, so pipeline policy and host ownership must also prevent mutation of released payloads. [Docker bind-mount semantics](https://docs.docker.com/engine/storage/bind-mounts/)

Changing a host symlink or release pointer does not update an existing container's pinned bind mount. Grant changes and release upgrades therefore recreate affected containers while preserving private volumes. Missing assigned storage fails startup visibly; never fall back to an ungranted release or expose the entire dataset store. Sandbox reset recreates only private data and reattaches current grants.

For removal, reject new attachments immediately, close affected sessions within 30 seconds, stop their containers, and recreate them without the removed bundle. For FEAM withdrawal, prepare a successor release using `withdraw` so tombstones persist; drain affected old snapshots and prohibit assignments/rollback that would reactivate withdrawn versions. If bytes must become inaccessible through direct shell reads, remove that bundle's mount as well; a tombstone alone governs FEAM access, not filesystem reads. Already staged copies or exported data cannot be recalled merely by withdrawing a product or removing a mount.

Retain at least the current and previous permitted release for rollback when capacity allows. Keep any additional release still referenced by a workspace or required recovery record. Garbage collection requires zero live assignments and no active preparation job, respects withdrawal restrictions, and uses recorded release IDs. Back up irreplaceable inputs/releases; a URL alone is not a reproducible backup.

## Phase 1: powerful hosted assistance

Phase 1 is a quality-oriented demonstration and observation period. The owner selected **DeepSeek V4.1 Flash through OpenRouter**, model ID `deepseek/deepseek-v4.1-flash`. Reuse the evaluated `deepinfra/fp8` provider pin, disabled provider fallbacks, and no automatic retries. Model selection is settled; rechecking current endpoint availability and validating the new broker integration remain necessary.

The repository's [fresh Stage-1 acceptance](tui_agent_stage1_acceptance.md#final-fresh-acceptance) records **97/100 tasks**, 169 requests, 90 tool actions, 2,436 ms median task latency, and no detected unauthorized actions in completed state/disclosure assertions. The [report](evaluations/stage1-2026-09-23/live-heldout-v3.json) records US$0.0163681504 known cost. This is bounded, historical, synthetic-fixture evidence for the FEAM harness, not proof of arbitrary scientific workloads, multi-user broker isolation, Ubuntu capacity, current pricing, or cloud recovery. Earlier failed/regression attempts and unknown-cost cases remain in the acceptance record; do not replace that history with the best score.

The assistant should demonstrate discovery, explanation of reuse metadata, clarification of ambiguous requests, pinned resolution, reviewed staging, publication-draft preparation/validation, withdrawal proposals, and recovery guidance. It uses the current [Stage-1 tools and boundaries](tui_agent_stage1_contract.md). It cannot administer users, grant datasets, execute arbitrary shell code, bypass local reviews, or modify a shared provider. More powerful inference does not change those permissions.

Start from the [evaluated profile](tui_agent_stage1_acceptance.md#live-evaluation-and-all-attempt-accounting): temperature 0, reasoning disabled, 1,024 output tokens, 48,000 context characters, 16,000 response characters, 128 KiB request/response bounds, eight tool calls, and a 120-second request envelope. The recorded price ceilings were US$0.14 input / US$0.42 output per million tokens; these are historical profile caps, not a new price quote. Recheck them before traffic and fail visibly if the pinned route is unavailable or exceeds an approved ceiling. The upstream route did not support the optional `parallel_tool_calls` parameter; preserve the tested omission and serialized local execution. Change settings only as a versioned, re-evaluated profile; do not silently route to a different provider or cheaper model.

### Model connection and credentials

Run a private inference broker outside user containers. It owns the real upstream key and accepts only the FEAM model protocol, with approved tools and a server-selected model/provider. It enforces account status, workspace ownership, disclosure rules, payload limits, and budgets independently of settings a shell user can edit. It provides no arbitrary HTTP proxy, filesystem access, or model administration API.

Keep demo containers on `--network none`. A small in-container adapter exposes an HTTPS loopback endpoint and forwards requests over that workspace's private Unix socket to the broker. Install a scoped trust certificate for the adapter in the image; keep both links private and preserve the current HTTPS profile requirement. The proposed `phase1-demo` profile points to this endpoint and obtains a short-lived **broker capability**, not an upstream API key, through the named environment variable. A shell user can read that capability, so bind it to their workspace/account, allowed operation, authorization version, expiry, and budget. Do not treat it as a secret that can grant more authority than that user already has.

The controller provisions the socket and capability and revokes them on stop, reset, or account disablement. Only this workspace's socket directory is mounted, with bounded storage and restrictive permissions. The adapter/broker must preserve streamed responses, complete tool-call arguments, provider request IDs, usage frames, cancellation, and typed errors expected by `mesh_agent`. This bridge is new implementation work and needs a real end-to-end compatibility test. The existing adapter follows the [router tool protocol](https://openrouter.ai/docs/guides/features/tool-calling); enforce provider restrictions in the broker using the [routing controls](https://openrouter.ai/docs/guides/routing/provider-selection).

Separate conversation histories and request IDs per user/session; never reuse one user's context in another's request. The broker records and verifies the actual provider/model used, rejects unapproved fallback, and redacts credentials and disallowed paths/content before persistence or external transfer. Preserve the existing harness's outbound filter as well. Pipeline-approved metadata may be included according to its disclosure policy; mounted datasets are not automatically uploaded. Select provider retention/data-collection settings explicitly; the router documents that policies vary by provider. [Provider logging policies](https://openrouter.ai/docs/guides/privacy/provider-logging)

### Spend and concurrency

Proposed starting admission limits are one in-flight model request per user and three across the host, with a visible fair queue. Ten users can have active assisted sessions while inference requests queue; ten simultaneous provider generations are not assumed. Cancel queued requests when their session expires, and verify account/grants again before dispatch.

The confirmed allowance is **US$100 across the project**, not per user, per host, or a renewing monthly allowance. OpenRouter credits are USD-denominated. Alert at proposed 75% and 90% thresholds; pause new dispatch whenever finalized spend plus outstanding/unknown reservations plus the new request's worst-case cost would exceed the current allowance. Existing admitted requests remain reserved and reconcile normally. An authorized admin can raise the total ceiling through an audited action; restarting, resetting a workspace, changing month, rotating keys, or moving to cloud never replenishes it. Show the pause and remaining/reserved amount clearly; manual FEAM remains available without being counted as hosted success. [OpenRouter credit currency](https://openrouter.ai/support/)

Enforce per-request and configurable per-user/day guardrails inside that project allowance. Reserve worst-case request cost transactionally before dispatch, including billed reasoning/output and applicable usage fees, then reconcile with returned usage. Uncertain billing retains its reservation and is marked unknown until reconciled; timeouts, disconnects, and client retries must not reset spending. Cache/reasoning token accounting and model prices belong to the pinned profile. Local `:usage` is a useful display, not the shared spending authority. Use a dedicated provider key/account limit as a secondary safeguard; do not assume an account balance shared with other projects is this project's ledger. Reconcile existing attributable project charges when initializing it. Track credit-purchase fees/taxes separately from metered usage rather than treating the US$100 credit allowance as an all-inclusive infrastructure budget.

The model, provider route, project currency/allowance, and pause-until-admin-increase behavior are settled. Credentials, current prices, per-request/user guardrails, and durable cross-host accounting must be configured before traffic. Missing or unreconciled billing state keeps assisted dispatch not-ready. Authoring this document does not run inference, add credits, or authorize a new billable evaluation.

## Usage capture and Phase 2 SLM development

Collect structured FEAM interactions so Phase 2 can learn what users actually attempt, where assistance works, and which tasks need a smaller model. Keep the same FEAM tool/service boundary for hosted and future local inference, as described in the [TUI/agent design](../tui_agent_harness_design.md). Raw terminal transcripts, broad keystroke recording, and the recovery journal are poor substitutes for structured workflow events.

| Event/data | Capture | Purpose |
| --- | --- | --- |
| Context | Pseudonymous participant ID, session/turn/task IDs, existing-consent record reference, FEAM image/tool-schema/profile versions, dataset bundle release and manifest revisions | Reconstruct the relevant environment without account email or host paths. |
| Request and response | Redacted user request and visible assistant answer, clarification turns, explicit edits/corrections | Identify intents, helpful explanations, ambiguity, and repair patterns. |
| Tools and reviews | Proposed tool/arguments, validation result, filtered tool result, review presented, accepted/rejected/edited/cancelled decision | Distinguish suggestions from approved actions and learn safe tool selection. |
| Outcomes | Commit/receipt or journal outcome where verifiable, typed failure, abandonment, manual fallback, optional user rating | Measure task success independently of model claims. |
| Usage | Observed model/provider, provider generation ID, input/output/available reasoning/cache token counts, reserved/final/unknown cost, queue/first-token/total latency | Compare quality, cost, speed, and future SLM requirements. |

Capture model exchange/accounting at the broker and application actions at explicit TUI/harness/service instrumentation points. Application events use a separate per-workspace Unix socket, so collection needs no general sandbox network access. Apply the same bounded-directory permissions and revocable workspace identity as the model bridge; expose only event submission, not trace queries or exports. The collector correlates IDs, deduplicates retries, orders events, and records missing spans. Reviews and later user actions cannot be inferred from broker traffic alone. Existing `:usage` and journal records do not contain this complete sequence; event emission and collection are new work.

Store bounded, sanitized events in a private append-only event stream with a small local SQLite index and a separate transactional broker budget ledger. Keep these stores outside sandbox/reset volumes. The collector authenticates event sources and enforces quotas, schema versions, and record-size limits. Mark broker-observed, client-reported, and independently verified fields separately: sandbox users can alter their software and fabricate application events. Verify important success labels against applicable manifests/receipts where possible; a reported success is not automatically a training target. Capture gaps must be visible in admin metrics and exclude incomplete examples from trusted exports. Ledger failure rejects new billable requests; telemetry failure visibly pauses recorded sessions or requires an explicitly labeled unrecorded mode, with no silent loss presented as complete evidence.

The owner confirms that participant consent already exists and no new participant notice is required. Record that administrative confirmation and a reference to existing consent evidence where available; do not fabricate an original consent date, signature, or version. Do not add a first-use notice or repeat-consent dialog. Continue to display provider/recording status and honor withdrawal from future research collection. An unrecorded/manual path can remain available under the participation policy; label it and do not fabricate missing research data. Keep minimum operational accounting separate from research content. Proposed retention remains 90 days for sanitized session events and 30 days for operational logs; curated exports need an explicit owner, purpose, expiry, and deletion policy rather than indefinite retention. Approval of workspace retention does not silently settle a different research-retention policy.

Exclude API keys, auth tokens, emails, private paths, raw dataset payloads, and hidden model reasoning from the research corpus. Store only visible responses and structured actions/outcomes. Maintain the participant-to-account mapping separately with restricted access, and require a separate research-export permission for trace content. Process participant deletion/withdrawal across active events, derived datasets, and expiring backups; record export lineage and the practical limits of retracting already distributed artifacts or trained models. Review dataset/model-output reuse conditions before treating traces as training material; demonstration permission and provider-side no-training settings are not the same as permission for our own training reuse.

Phase 2 uses the collected evidence in this sequence:

1. Summarize real task frequencies, clarification needs, failure/denial patterns, manual corrections, and latency/cost. Use these findings to set the SLM's supported scope and improve deterministic tools or UI where that is more effective.
2. Build versioned, reviewed examples from eligible traces: task context, user request, allowed tool choices, validated arguments/results, concise answer, and verified expected outcome. Keep rejected/failed proposals with explicit negative labels; never convert them into successful demonstrations. Retain provenance to source events, model/profile, consent, dataset release, and reviewer.
3. Deduplicate and split by participant/project, dataset family, and task template to reduce leakage. Freeze a held-out evaluation set before prompt tuning or training, and keep it out of teacher-generation prompts and training exports. Report the small cohort's coverage limitations and label synthetic augmentation separately.
4. Benchmark candidate SLMs against the powerful hosted baseline using the same supported tools, fixtures, and task-outcome rubric. Try prompt/context/schema improvements first; use eligible examples for supervised adaptation/distillation if justified. Report completion, clarification quality, denied/unauthorized actions, tool validity, latency, and CPU/RAM footprint. Model outputs alone are not correctness labels.
5. Train or fine-tune in a separate development/training environment; it is not an admin workspace on the demo host. Version the resulting model, prompt/template, quantization, tool contract, and evaluation report. Promote an SLM only after it meets agreed quality and target-resource gates, preserving the same permissions and review semantics. Keep hosted Phase 1 and local/HPC Phase 2 results distinct.

## Capacity, storage, and admission

The following numbers are proposed initial limits, not measured consumption.

| Workload | Count | Memory hard limit | CPU ceiling | Persistent storage |
| --- | ---: | ---: | ---: | ---: |
| Regular demo | Up to 10 | 512 MiB each | 0.5 logical CPU each | 2 GiB each |
| Dataset-loading worker | At most 1 | 2 GiB | 1 logical CPU | 50 GiB staging budget, reserved before each job |
| Tunnel, gateway, controller, inference broker, collector, databases | Shared | Combined budget 2 GiB | Bound background work; prioritize interactive traffic | 5 GiB operational storage plus 20 GiB trace-store budget |
| Shared dataset releases | Shared across granted sandboxes | File cache accounted in measured host/container usage | I/O bounded by job and reader limits | 100 GiB initial retained-release budget |
| OS, desktop, page cache, reserve | Shared | Approximately 7 GiB remains on a 16 GiB host | Remaining capacity | Existing host storage |

Ten demo limits total 5 GiB; adding one dataset worker and the control plane yields about 9 GiB. Hosted model inference runs remotely, so Phase 1 reserves no local model weights or GPU. Account for runtime overhead and memory already used by other host applications. The controller admits a new workspace or pipeline job only when both the configured aggregate budget and current host headroom permit it; show a queue or capacity message otherwise. Model-request admission and spending reservations are separate broker checks. Do not rely on every process staying below its limit by chance.

Set 128 PIDs per demo container, bounded file descriptors, and size-limited `/tmp` and `/dev/shm`; tmpfs usage counts against the memory budget. Disable additional container swap allowance so a runaway sandbox cannot create a host-wide swap storm. Add an aggregate demo cgroup ceiling, and retain host headroom even if every user reaches their cap. A 512 MiB limit is for the TUI, hosted-request handling, and bounded fixture queries; it does not make arbitrary large Polars/raster processing safe. Measure with the actual approved model profile and dataset workloads before increasing demo scope.

Rootless Docker resource flags require cgroup v2 and systemd, with appropriate controller delegation. Verify `memory`, `cpu`, and `pids` enforcement by measurement, not just accepted configuration. Ubuntu's restrictions on unprivileged namespaces may require the supported runtime/AppArmor packaging. [Docker resource controls](https://docs.docker.com/engine/security/rootless/tips/), [Ubuntu rootless prerequisites](https://docs.docker.com/engine/security/rootless/troubleshoot/)

Store service data in a dedicated directory on the large home filesystem, such as `/home/feam-service-data`, with owner-only subdirectories. Verify its mapping to the owner-confirmed 2 TB SSD. The actual location is an operator deployment choice; it must not expose `/home/saif` to containers. Container memory limits do not enforce disk quotas. Ansible provisions the fixed-size ext4 workspace pool and mount units specified in [Storage provisioning contract](#storage-provisioning-contract); preallocate storage rather than relying on unbounded sparse growth. The web controller cannot mount arbitrary images or paths. A dedicated quota-enabled filesystem is a later alternative; existing ext4 quota support has not been verified.

Maintain the two preallocated reset spares, serialize reset jobs if needed, and include spare volumes, Docker image storage, logs, and backup scratch in the disk budget. Container root filesystems remain read-only so persistent writes cannot bypass the assigned workspace limit. Bound and rotate terminal/runtime logs; disable shell transcript logging by default. Refuse new allocations/starts/reset jobs when the underlying host data filesystem or root filesystem has less free space than 15% or 20 GiB, whichever is larger. Apply per-volume byte/inode and headroom checks separately: the 20 GiB host threshold must not be applied to each 2 GiB workspace image or other small bounded filesystem.

Put dataset staging and releases on the same dedicated, size-bounded filesystem to support atomic rename promotion. Provision 200 GiB for the proposed 50 GiB staging and 100 GiB retained-release budgets plus filesystem overhead and free-space reserve. Before a job, require that actual available space after its complete reservation stays above the dataset filesystem's 15% or 20 GiB threshold, whichever is larger. Reserve the complete candidate, incoming source, and required rollback versions; refuse a job that cannot fit even if one logical budget has room. Keep the trace store independently bounded, with its own high-water mark, retention, and visible backpressure rather than silent event loss. Dataset retention and research-event retention are separate from the 30-day private-workspace policy.

Prefer preprocessed, modest-size dataset subsets for the initial cohort, with larger inputs added only after measurement. Stream validation and conversion where possible, cap ingestion I/O, and schedule heavy jobs outside advertised demos if they affect interaction latency. Builds and SLM training run in the project's separate engineering environment or available CI; the web service hosts management, demos, bounded ingestion, and telemetry.

## Software and model-profile releases

FEAM software releases, dataset releases, and hosted model profiles are independently versioned. Loading a dataset does not require rebuilding the FEAM image. Record all three in session events so comparisons and rollback remain interpretable.

Release workflow:

1. The engineering pipeline builds and tests an image with the hosted feature, fixtures, SDK, terminal tooling, and broker adapter. Record source commit, dependencies, schemas, and image digest.
2. Test it in a disposable staging sandbox through the actual browser, broker, and collector path, including the selected powerful model and assigned dataset release. Run live checks only with the chosen account and budget configured.
3. Mark the image and model profile as approved. Admins select approved versions and see their compatibility and expected cost; they do not execute build scripts from the dashboard.
4. Start new sessions with the approved image/profile and current grants. Model-profile changes begin a new conversation context and clear pending proposals, preserving current TUI review semantics.
5. Drain sessions before software migration. Keep the previous compatible image and data backup for rollback. Apply current account/grant/withdrawal rules after any restore; a rollback never restores revoked access.

Host-side automation pulls or installs approved artifacts with narrowly scoped credentials. The admin website and user containers receive no runtime socket, arbitrary deployment authority, or upstream model key. Pipeline/collector restarts must not restart unrelated sandboxes, and broker restart must preserve spending reservations and cancellation state.

Ansible's release deployment drains affected operations, takes a consistent pre-migration backup, runs explicit versioned migrations once, and verifies health before marking the release current. Unit/configuration changes notify only affected services. Retain the approved image and host binaries on disk so an ordinary reboot does not need the build system or registry. A previous binary is a valid rollback only with a compatible database schema; otherwise use the explicit restore workflow and its current-authorization/billing reconciliation. Bootstrap, migrate, and restore are separate recorded actions.

## Lifecycle and control data

Use a small local SQLite database on the local filesystem. Proposed records:

| Record | Minimum fields |
| --- | --- |
| Account | Immutable ID, exact email, verified provider identity, role, pending/active/disabled state, authorization version, timestamps. |
| Workspace | ID, owner ID, active-site ID, hostname, assigned image digest/model profile, dataset grants and release assignments, recorded volume/socket IDs, site-local generation, state, last activity. |
| Lifecycle job | ID, action, workspace ID, idempotency key, expected generation, status, bounded error detail. |
| Dataset job/release | Job/source IDs, definition hash, bundle namespace, parent release, candidate/release hash, source provenance, checksums, validation/approval state, size, manifest revision, withdrawal restrictions. |
| Dataset grant | Bundle and selected release, audience/user/workspace IDs, status, grant version, effective time, actor. |
| Model request/budget | Account/workspace/request IDs, profile, reservation, observed usage/cost or unknown state, queue/dispatch/completion times, provider generation ID. |
| Research consent/export | Pseudonymous participant mapping, existing-consent record/state, event references, corpus/schema version, retention, export provenance and permissions. |
| Audit event | Time, actor ID, action, target ID, outcome, correlation ID; no credentials or terminal contents. |

Serialize start/reset/delete per workspace and reserve capacity transactionally to prevent duplicate containers from concurrent requests. Reconcile database state against container labels on controller restart; clean up orphan runtime resources only when ownership is proven. States should distinguish `stopped`, `starting`, `running`, `stopping`, `resetting`, `failed`, and `pending-delete`.

Browser disconnect should preserve a short reconnect window, using a private `tmux` session if persistent terminals are desired. Count user input or explicit activity, not transport heartbeat traffic, for idle stopping. Track an active model/tool operation explicitly so it is not mistaken for idle transport, but bound its lifetime. Cancel queued/in-flight inference as appropriate on stop and reconcile any billable work. Warn before idle shutdown. Stop gracefully so FEAM can finish or journal an in-flight mutation, then enforce a shutdown timeout. An idle stop preserves files; reset replaces private demo data and reattaches current grants; account disable revokes sessions and broker capabilities and stops workloads. Dataset and trace retention follow their own policies.

## Restart and reboot contract

The intended result is automatic recovery of the service and persistent data, with visible reconnection where processes are interrupted. The following are acceptance requirements, not measured results:

| Event | Expected recovery | What the user can expect |
| --- | --- | --- |
| Browser/tunnel/gateway disconnect | Reconnect to an existing eligible container and private `tmux` session within its activity window. | Terminal connection interruption; surviving processes can continue. Revalidate identity and ownership. |
| Controller restart | Inspect recorded container labels, slot IDs, generations, grants, and pending jobs; adopt only proven owned resources. | Existing eligible containers keep running; lifecycle controls pause during reconciliation. |
| Broker/collector/pipeline restart | Recover durable requests, reservations, event cursors, and jobs independently. | Affected operations show interruption or an unknown outcome; do not automatically repeat a billable request or uncertain publication. |
| Host reboot or rootless-runtime restart | Mount/verify storage, recover services, reconcile state, and recreate eligible workspaces from pinned artifacts. | Files persist; terminal/TUI processes and in-memory pending reviews do not. Users reconnect and review recovery state. |
| Host loss | Start the cloud replacement from approved artifacts, current control state, and prepared datasets. | Fresh cloud workspaces and sessions; disclose that private Ubuntu files are not transferred. Preserve accounts, grants and spending. |

Use independent service units with bounded restart delays/rate limits, startup health checks, and graceful shutdown timeouts. The rootless runtime is enabled under the dedicated user's boot-started manager. Disable Docker's autonomous restart policy on demo containers so only the controller starts them after authorization/storage checks. Gateway/controller failures must not cascade through unrelated sandboxes; state or mount failures deliberately stop affected writes.

Boot in this order:

1. Verify the underlying filesystem and mount the registered data/workspace filesystems, recreate bounded socket tmpfs directories, and verify UID/GID mappings and ACLs. Missing storage fails service startup visibly; it never initializes an empty replacement database or dataset tree.
2. Start the runner's user manager/rootless daemon and independent host services after their storage dependencies. Open existing databases at the expected schema version; perform recovery checks without replaying deployment migrations.
3. Confirm this host is the active deployment, reconcile containers/jobs against durable records, invalidate stale capabilities, and preserve all unknown spending/publication outcomes for reconciliation. Readiness remains false if authorization, billing, or required recording state cannot be trusted.
4. Admit eligible workspaces within current capacity and grants. Keep explicitly stopped/disabled workspaces stopped; interrupted reset/import jobs go through their recovery rules before resuming. Reissue workspace capabilities and start new TUI conversations without pending approvals from before the reboot.
5. Report assisted-demo readiness through the authenticated gateway. The tunnel may connect earlier to show maintenance, but terminal/model routes remain unavailable until their checks pass. Failure of one assigned release blocks its workspaces; failure of shared authorization/billing storage blocks all affected routes.

A proposed initial target is readiness within five minutes after the OS, required storage, and network are available, with no interactive host login. Measure it on the Ubuntu machine and revise before making a user promise. Hardware power restoration, failed disks, firmware prompts, provider outages, and full-disk recovery fall outside that routine-boot target.

## Cloud portability and recovery

The software has a straightforward **single Ubuntu VM** deployment path: use the same `linux/amd64` images, host binaries, Ansible roles, private sockets, and container mount paths, with a cloud inventory supplying the data-volume mount and host identity. Native rootless Docker needs a VM with a compatible kernel/systemd and storage/mount privileges; a restricted serverless container platform is not an interchangeable target. ARM is a separate build/test target and should not be selected solely for a lower VM price.

The owner explicitly permits losing private workspaces when moving between Ubuntu and cloud. Do not implement cross-host workspace mirroring, live container migration, or workspace-volume restore as prerequisites for cloud readiness. Keep SQLite and active workspace/dataset files on local block storage; object storage holds encrypted control-state checkpoints, research backups and immutable recovery artifacts, not a live filesystem for SQLite or FEAM's direct reads. A cloud copy of the personal Ubuntu disk is unnecessary.

This does **not** make the service stateless. The same identities, roles, grants, dataset releases, revocations, consent/export restrictions, project allowance and known/unknown spending must govern either host. Keep that small control/recovery state current off-host, independently of disposable workspaces. Preserve existing research data under its retention policy; workspace reset is not permission to erase the corpus or billing ledger.

| Cloud fallback mode | Standing resources and cost | Recovery tradeoff |
| --- | --- | --- |
| Fast cold start, revised candidate | Prebuilt VM image, prepared seed release, encrypted control/research recovery material, state and off-host runner; create full-capacity compute when needed. | Planning estimate **5–15 minutes** with artifacts/data ready; no workspace restore. Allocation, runner scheduling, initialization and routing can exceed this. |
| Plain Ubuntu cold start | Same independent recovery material, but install the runtime and fetch/configure artifacts after VM creation. | Planning estimate **15–30+ minutes**; network/package/registry delays make this unsuitable for a strict short deadline. |
| Stopped prebuilt VM | Retained disks/snapshots, addresses, and possibly compute depending on provider. | Avoids some setup/allocation work; must refresh control state and validate readiness. Benchmark provider start behavior. |
| Running standby | Full-capacity VM plus persistent storage and continuously checked artifacts/control state. | Lowest startup delay at recurring compute cost; still requires fencing, promotion checks and restarted sessions. Use if cold-start drills miss the accepted target. |

These time ranges are **engineering estimates, not measured results or a provider guarantee**. On 2026-09-24, the owner accepted a cold-start recovery target of **at most 15 minutes from detected Ubuntu failure to authenticated assisted-service readiness**, including recovery-runner start, fencing, compute allocation, data/artifact availability, fresh workspace initialization, budget reconciliation and route changes. Include detection delay separately in end-to-end outage reporting. The recovery-time decision is settled; the cloud spending ceiling and deployed timing evidence remain pending. A cold VM cannot guarantee spare capacity during a provider incident. If repeated drills cannot meet the accepted target, revisit the image/data preparation or propose a priced warm-standby alternative for approval rather than silently relaxing the target or incurring recurring compute charges.

Do not assume powering a VM off eliminates its bill. For example, AWS EC2 stops charging on-demand instance usage while stopped but continues charging retained EBS storage; Hetzner charges for an existing server even when it is powered off. A snapshot-and-recreate strategy may therefore be cheaper than a stopped server. Compare the chosen provider's complete bill before provisioning. [EC2 lifecycle billing](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-lifecycle.html), [Hetzner billing](https://docs.hetzner.com/cloud/billing/faq/)

Use **16 GiB RAM and 8 vCPUs as the initial full-service sizing candidate**, subject to actual CPU architecture, sustained performance and I/O measurements; this is not measured equivalence to the Ryzen host. Keep ten simultaneous lightweight sandboxes, the same inference queue/model, one bounded ingestion worker, dashboard, telemetry, retention and permissions. Pause ingestion only during promotion checks, not as a permanent cost-saving downgrade. Hosted inference needs no local GPU. Prefer a Toronto or Montreal-area Canadian region for the Ottawa cohort; this is a locality proposal, not a claim of institutionally required data residency. Test terminal responsiveness from actual Ottawa user networks.

Cloud storage must enforce the same service quotas even though initial data is small. The local 2 TB SSD does not require renting a 2 TB cloud disk: approximately 269 GiB of service allocation plus the 15% underlying-filesystem reserve requires roughly **320 GiB of service storage**, in addition to the OS and any extra backup scratch. Validate exact provisioning and filesystem overhead. Empty capacity can be allocated on promotion; it need not contain 320 GiB of restored data. Keep the initial prepared climate bundle small and immutable, with all assigned release bytes already recoverable independently of Ubuntu. Never reduce the ten-user quota or permanently disable ingestion to claim equivalent service on a smaller VM.

Do not fetch government sources, compile FEAM, or download the entire retained dataset history on the outage path. Prebake runtime packages/binaries/images, refresh the approved image on releases, and keep assigned dataset releases region-local or pre-positioned on a retained volume. Admit a newly assigned release only after its recovery copy is verified. As actual assigned datasets grow, measure restore bandwidth and retain a ready data volume if necessary to preserve the minutes target. Standing cost comprises images/artifacts, control/research recovery storage, optional ready volume, recovery runner and API/network charges; full compute/storage during fallback and drills are additional. Select provider/region and a monthly ceiling before provisioning; the US$100 model allowance does not authorize cloud charges.

### Consistent recovery material

Build a recovery manifest that records backup time/ID, schema/software versions, image digests, model profiles, account/grant/consent revisions, budget checkpoint, event watermark, UID/GID mappings, and release IDs/checksums. Workspace slot/generation backups may support same-host recovery, but cross-host promotion deliberately creates new site-local generations. A recovery point is complete only when all required objects are verified in independent storage. Retain seven daily encrypted backups for rollback and propose incremental control/research checkpoints at least every five minutes; nightly-only state must not be presented as a current failover checkpoint. Bound state size and measure checkpoint/finalization time before setting a non-workspace data-loss promise.

During a bounded control-plane checkpoint, pause lifecycle/pipeline commits and model dispatch, settle or record uncertain requests, and flush event collection. Back up each SQLite store through its backup API or a tested clean-stop procedure; coordinate the stores because separate SQLite backups are not a cross-database transaction. Ordinary shell processes need not stop for this control-only checkpoint: their files are outside the cloud-continuity contract. If separately backing up private workspaces, stop their writers and preserve ownership, symlinks and ACLs; raw copies of live databases or writable image files are not consistent backups. Retain immutable dataset releases by checksum, verify references, and use incremental encrypted off-host storage with bounded local scratch. [SQLite backup API](https://www.sqlite.org/backup.html)

Retain the proposed seven daily recovery points, applying participant deletion/retention policy. Cross-host workspace recovery is explicitly not promised; users export important results themselves if desired. Exclude runtime sockets, expired tokens and disposable caches. Keep approved binaries/images, prepared datasets, and decrypt/restore credentials available even if Ubuntu and its registry access are lost. Keep the same active-host workspace retention policy, but clearly explain that a site switch starts a different workspace and cloud work need not be carried back.

Backups can be older than an account, grant, revocation or billing event. Persist versioned control changes and the active-site generation in a restricted off-host recovery journal before reporting them durably complete. Enforce restrictions locally immediately even when synchronization fails, report them pending, and refuse affected restored access until completeness is established. Persist each model reservation off-host **before dispatch**, using the project request ID and activation generation; persist reconciled cost and allowance increases as well. A journal outage therefore pauses new billable requests rather than creating an untracked spending window. Restore a checkpoint plus its verified journal tail idempotently; unresolved costs retain reservations. Provider request IDs support later reconciliation but a slow provider lookup must not release reserved money. Prove the protocol under crashes between local commit, remote acknowledgment, dispatch and cost finalization.

Historical sanitized research events remain in encrypted off-host archives with checksums, retention and deletion lineage; they need not all be copied back before users can open fresh sessions. Restore the trusted capture watermark, current permissions and bounded recent index/tail, then archive or hydrate older events separately. Report any missing span honestly. Define and test a bounded startup-state manifest so a growing trace archive or audit database cannot silently turn the minutes-scale cold start into a bulk restore.

### Promotion and return to the Ubuntu host

Use separate local and cloud tunnels. Two active connectors for the **same tunnel** may both receive traffic; replicas do not provide an ordered primary/standby routing policy. Nor do they replicate SQLite or budgets. Keep public hostnames/Access audiences stable and switch targets only after replacement checks. The bounded failover controller owns active targets; Terraform owns resources and must not revert those runtime targets on apply. [Cloudflare tunnel replica behavior](https://developers.cloudflare.com/tunnel/configuration/)

1. Detect loss using an independent monitor. Fence the old deployment by stopping it or proving equivalent denial of public access and external mutation credentials, including policy reconciliation and inference. A failed health probe alone is not fencing. Use a strongly consistent off-host activation lease/generation, continuously checked by gateway, controller, pipeline and broker, not only at boot. Loss/expiry closes sessions and stops affected work; a stale host cannot rejoin from cached state. Record and test lease timeouts, renewal and paused-process behavior. If fencing cannot be proven, stop promotion and alert instead of allowing two authorities.
2. From an off-host runner, execute the approved locked cloud Terraform plan using the prebuilt image, then apply the shared Ansible recovery roles. Avoid an operator-laptop or manual-approval delay on the timed path: preauthorize only this bounded recovery workflow when deployment is enabled. Keep admission, pipeline dispatch, membership reconciliation and spending disabled during recovery. Use a private validation path.
3. Load the bounded startup-state checkpoint/journal and prepared assigned datasets, verify hashes and database integrity, replay later authorization/billing events, and rotate capabilities/host-specific credentials. Verify historical research archives remain recoverable without bulk-hydrating them on the outage path. Allocate fresh cloud workspace slots for active regular users and initialize practice fixtures/current grants. Do not recreate interrupted private-file mutations or replay pending approvals from Ubuntu; cloud sessions start new conversations.
4. Validate authentication/roles, the same datasets and quotas, terminal sockets, the pinned hosted route, project-wide budget and collection. Confirm the new activation generation, switch the fixed portal/admin/workspace tunnel targets through the single route owner, and reconnect users to fresh workspaces. Record outage duration, recovery-state age, charges and any failed checks. The next Terraform apply must preserve the promoted site.
5. Return to Ubuntu through a controlled handback, not automatic failback. Checkpoint current cloud control/research state and dataset changes; fence cloud operations and reconcile that current state onto Ubuntu. Reuse retained Ubuntu private workspaces only after current grant/revocation checks and container recreation, or initialize new ones. Cloud private files need not transfer, and must never overwrite retained Ubuntu files implicitly. Start new conversations, advance activation generation, validate and switch routes. Release cloud compute after verified non-workspace recovery material and handback; disclose loss of cloud-local workspace files before retiring their storage.

The earlier 24-hour workspace RPO and one-working-day RTO are superseded. **Private workspace continuity is not required; the accepted service-recovery target is at most 15 minutes after failure detection.** Meeting it depends on prebuilt artifacts, small/pre-positioned assigned data, current control state, available VM capacity, an independent runner and successful fencing, and remains unverified. Do not put a full bulk restore on this path: even 100 GiB at sustained 100 Mbit/s takes about 2.4 hours before validation. Cloud fallback covers local power/internet/hardware loss; Cloudflare and the selected model provider remain shared dependencies. Failure to reconcile identity or billing is a visible recovery failure, not permission to bypass controls to hit a timing target.

## Operations and recovery

- **Reachability:** the host must remain powered, awake, and connected during advertised demo hours. Use stable wired networking when available. Check tunnel egress, approved pipeline/model egress, DNS, IPv4/IPv6 firewall behavior, and recovery after reboot. No public Docker, database, raw terminal, inference, or SSH listener is required.
- **Supervision:** Ansible installs the mount, system-service, and rootless user-service configuration in the [restart contract](#restart-and-reboot-contract). systemd starts the tunnel, gateway, controller, pipeline, broker, collector, and rootless runtime independently. Fail closed when required mounts, activation state, authorization, or spending storage are unavailable. A local-only health endpoint reports assisted-demo readiness, including broker/provider health and collector capacity, without exposing secrets.
- **Monitoring:** show active sessions, resource/OOM/disk use, grant/policy sync, pipeline failures, model queue length, provider errors, known/unknown cost, budget remaining, event capture gaps, and backup age. An external availability check is needed to detect complete host loss; local monitoring cannot do that.
- **Audit:** retain bounded local admin and lifecycle audit logs for a proposed 30 days. Redact JWTs, email codes, cookies, secrets, and authentication URL queries. Do not rely solely on free-provider log retention.
- **Backups:** use the checkpoint, journal and archive contract in [Consistent recovery material](#consistent-recovery-material). Preserve control state, spending, consent/export records, private configuration, pipeline provenance, approved datasets and eligible traces independently of Ubuntu. Private workspace backups are optional for same-host recovery, not a cloud-continuity requirement. Keep seven daily encrypted points plus bounded incremental state; backup credentials remain inaccessible to workspaces.
- **Recovery target:** owner-accepted maximum of 15 minutes after failure detection for cloud service recovery with fresh workspaces, as described in the [promotion procedure](#promotion-and-return-to-the-ubuntu-host). Verify it in deployed cold-start drills before advertising achieved recovery performance. Reconcile current grants, research deletions and billing, invalidate old capabilities and fence the previous authority. Never reset the US$100 allowance on site change. A separate five-minute readiness target applies only to routine boot after its dependencies become available.
- **Dependencies:** internet, Cloudflare, power, or host loss makes browser access unavailable. Hosted-provider loss prevents the intended assisted demo even if manual FEAM remains available. Preserve private operator recovery access; auth or provider outages must not trigger an unapproved bypass or model fallback.
- **Data handling:** browser traffic passes through Cloudflare; filtered model context passes through the selected router/provider. Shared datasets and research traces remain on the active host, with approved encrypted backups/exports elsewhere. Cloud storage and recovery processing must be allowed by the selected dataset, participant, and region policies. Apply each loaded dataset's audience, disclosure, and research-use policy.
- **Maintenance:** patch the OS/runtime through reviewed Ansible changes and rebuild images on a scheduled basis; announce planned restarts. Store an operator runbook covering preflight, Terraform state recovery, converge/deploy/restore, policy reconciliation, release rollback, account disablement, disk/OOM recovery, and cloud promotion/handback. Test backup restore periodically and after storage/schema changes.

## Cost and alternatives

| Item | Baseline cost | Qualification |
| --- | --- | --- |
| Existing Ubuntu hardware | No new compute rental | Power, wear, and internet remain real costs. |
| Cloudflare Zero Trust Free | $0 subscription for this user count | Current limit is 50 users; no paid features assumed. |
| Tunnel connector and host software | No software subscription proposed | Docker Engine/rootless, ttyd, Go, SQLite, systemd; local pipeline/broker/collector implementation and maintenance still take time. |
| IaC tooling | No paid Terraform or Ansible subscription required | Terraform CLI and Ansible run from an operator machine; HCP Terraform is optional. Backend storage, CI/artifact retention, and maintenance may cost money. |
| Domain | New registration and annual renewal required | Numeric `.xyz` is the lowest-cost candidate found; verify exact checkout/renewal. Free GitHub Pages is only a possible separate landing page. |
| Backups | $0 incremental if existing separate storage is available | New disks or cloud storage may cost extra. |
| Email delivery | No separate SMTP service in the recommended path | Access supplies the email PIN flow. |
| Hosted model API | US$100 project usage allowance | DeepSeek V4.1 Flash through the pinned OpenRouter route; pause new dispatch until an admin raises the allowance. No monthly/host reset. |
| Dataset ingestion and trace storage | No managed service required | Source licensing/egress and any extra backup capacity may cost money; use approved local/public data where suitable. |
| Cloud fallback | Prebuilt-image/data/state retention and recovery-runner cost at rest; full compute/data-volume charges when allocated | Benchmark fast cold start with fresh workspaces. Include addresses, requests, transfer and drills; a powered-off VM is not universally free. Use a paid warm standby only if timing requires it. |

For electricity planning, measure wall power: average watts × operating hours ÷ 1,000 gives kWh; multiply by the applicable tariff. Do not present existing hardware as cost-free to operate. Avoid auto-upgrading services to paid plans; show quota failures explicitly and review limits before increasing scale. [Cloudflare plan limits](https://www.cloudflare.com/plans/)

Estimate model spending from measured requests per task, tasks per user, and billed tokens/fees for the pinned profile, including reasoning/cache behavior where applicable. The US$100 project allowance is settled; the very low historical fixture cost is not a forecast of arbitrary real sessions. Keep the selected model and pause at the admission boundary until an admin raises the allowance; do not silently substitute an unvalidated model. Recheck current route prices before rollout.

| Alternative | When it helps | Why it is not the starting point |
| --- | --- | --- |
| Supabase + free SMTP magic links | Clickable links are a firm requirement | More services, mail setup, callbacks, and free-project pause behavior. |
| Direct HTTPS reverse proxy on this machine | Public address/port forwarding already works; avoid a tunnel provider | Requires ingress and certificate operations, plus a separate identity layer. |
| Private VPN access | Small trusted internal team | Adds client setup; less convenient for a browser-only external demo. |
| Full workspace platform | Many hosts or richer workspace provisioning | More deployment scope than ten assisted terminals and a management dashboard. |
| VM or microVM per user | Users are mutually untrusted or need unrestricted code execution | Stronger boundary, with more memory and operational overhead. |
| Cloud VM | Recover from local host/power/internet loss using the same Ansible roles | Cold recovery is the proposed fallback; a running standby needs a separate cost and recovery-time decision. |

## Delivery milestones and acceptance

The owner decisions and private planning roster are recorded; the existing Stage-1 model evaluation is linked above. All web/IaC delivery milestones below remain future work. Document creation did not install services, register a domain, create external accounts, deploy infrastructure, download datasets, send invitations, or run a new live model/load test.

These milestones implement Phase 1; Phase 2 SLM development follows the evidence workflow above.

| Milestone | Deliverable | Exit condition |
| --- | --- | --- |
| 0. IaC foundation | Recorded Ubuntu preflight; Ansible accounts/storage/runtime roles; Terraform edge configuration; state/secrets recovery; locked artifact versions and CI | Safe first and repeat converge on disposable Ubuntu; mounts, existing-daemon coexistence, runner boot without login, and correct architecture verified. Live cloud/account resources remain an explicit deployment step. |
| 1. Local proof | Hosted-capable FEAM image, one rootless demo container, private socket terminal, fixed-size persistent volume | FEAM walkthrough works; reset is confined; actual resource limits and namespace permissions pass. |
| 2. Identity and gateway | Terraform-managed domain/tunnel/applications and static policies; separately reconciled email groups; validated identity, local ownership store, minimal portal | Unlisted identities are denied; two listed users cannot reach each other's HTTP or WebSocket endpoints; admin separation and no allowlist rollback after an infrastructure apply pass. |
| 3. Management and datasets | Lifecycle controller, quotas, policy reconciliation, bounded AAFC/ECCC importers, immutable GeoTIFF/Parquet releases and grants | Admins manage without a workspace; source provenance, conversion/CRS checks, real SDK/raster reads, updates, resets and revocation pass. |
| 4. Hosted assistance and evidence | Broker for the selected DeepSeek route/profile, project spend ledger, event instrumentation/collector, existing-consent records and export controls | Real hosted tool/review exchange, isolation, US$100 pause/admin-increase behavior, durable reservations and correlated traces pass before rollout. |
| 5. Ten-user acceptance | Concurrent assisted sessions with shared datasets, queued inference, bounded ingestion, and fault checks | Meets the gates below with actual machine/model evidence and capture coverage. |
| 6. Cohort handoff | Checkpoint backups, Ansible deploy/restore, pinned release/rollback, reboot recovery, operator/admin runbooks, curated Phase 2 export procedure | Restore/rollback and unattended reboot demonstrated on Ubuntu; cohort budget/model/recording policy set; initial export validated before SLM work. |
| 7. Cloud fallback acceptance | Provider/region/budget, prebuilt image, off-host runner, bounded startup state, Terraform/shared Ansible, fenced promotion and route ownership | Repeated authorized cold-start drills achieve authenticated assisted-service readiness within 15 minutes of failure detection and verify full ten-user service from Ottawa. Fresh workspaces are expected; current roles, grants, datasets and allowance survive. Test stale-host denial and handback. Until passed, this is a design, not an available 15-minute fallback. |

Required acceptance scenarios:

1. **Authentication and roles:** bootstrap the exact private four-admin/three-user roster with no public signup; repeat converge must not resurrect removed identities. Allowed email succeeds; unlisted email is denied; expired/replayed PIN fails; forged headers and wrong-audience JWTs fail. A regular user cannot open admin APIs, run dataset jobs, change grants/model budgets, or export others' traces. Admin provisioning allocates no workspace. Removal closes sessions within 30 seconds and revokes capabilities. Verify PIN delivery and browser/WebSocket access from the actual Ottawa institutional networks.
2. **Ownership and origins:** users A and B attempt each other's guessed hostname/IDs, reset endpoints, downloads if introduced, and WebSocket upgrades. All fail. Cross-origin/sibling-origin CSRF and terminal upgrade attempts fail. Sandbox-controlled responses cannot obtain portal authentication material. Direct origin/port access offers no authentication bypass.
3. **Isolation and quotas:** verify A cannot read/write B's private data, ungranted bundles, host home, pipeline staging, provider credentials, trace stores, other users' inference sockets, or host services. Exhaust memory, PIDs, CPU, disk, and event quotas separately; contain failure without starving other users. Shared mounts reject writes through shell and symlink paths. The model bridge cannot proxy arbitrary destinations or cross-account requests.
4. **FEAM and datasets:** run refresh, detail, pinned resolve, direct SDK reads, explicit private staging, and practice publication/withdrawal/recovery. Test successful load plus corrupt formats, missing metadata, namespace conflicts, duplicate versions, concurrent jobs, partial fetch/publication, and failed promotion. Grant a bundle only to A and verify B cannot read it even by path. Restart applies the selected release; removal rejects future access and stops old mounts; reset A leaves shared releases and B unchanged. Rollback cannot resurrect withdrawn access.
5. **Hosted quality and budgets:** complete a representative live benchmark with the selected DeepSeek route/profile through the actual broker. Use Stage-1 ≥90% task completion and zero unauthorized writes/disclosures as initial measured gates; report corpus/version, sample counts, failures, settings and finite-test limits. Test clarification, tool results, denial/edit, cancellation, malformed streams, rate limits, unknown cost and concurrent reservations. With a simulated ledger near US$100, prove admission pauses, unknown reservations remain charged, only an admin can increase the ceiling, and neither a calendar rollover nor failover resets it; no need to spend US$100 to test this. Client changes cannot lift limits/change route/reveal the key. Fake transport alone does not establish hosted compatibility.
6. **Usage evidence:** correlate a user's request through proposal, review/edit, confirmed outcome, feedback, model identity, latency, and known/unknown cost. Verify redaction, consent withdrawal, duplicate/missing events, client-report trust labels, collector failure, retention/deletion, and private export controls. A sandbox reset preserves eligible history but cannot bypass a participant's deletion request. Produce a small reviewed corpus with provenance and a held-out split; exclude hidden reasoning and unverified success labels.
7. **Concurrent load:** ten assisted sessions run a 60-minute representative workload with shared-data reads, staggered starts, and model/tool bursts. Proposed targets: warm start p95 ≤5 seconds, first initialization p95 ≤15 seconds, terminal input echo p95 ≤250 ms on a documented nearby connection, no host OOM, and no sustained swap thrashing. Record queue wait, first-token and full-task latency separately from UI responsiveness, plus per-container/host peaks, provider concurrency, costs, and event coverage. Calibrate acceptable model latency with the selected profile; do not promise a model completion SLA from local TUI numbers. Repeat with one bounded ingestion job and defer heavy jobs if it degrades the demo.
8. **Failure and recovery:** exercise disconnects, expiry, idle stop, gateway/controller/broker/collector crashes, provider outage, host reboot, disk-full, failed policy sync, interrupted reset/import, and corrupted release metadata. Preserve ownership and honest accounting/capture status. Demonstrate backup restore and independent software/model-profile/dataset rollback under current grants and withdrawal rules.
9. **IaC and boot:** validate a repeat converge preserves user files, database balances, identities, grants, and unrelated Docker workloads; no repeated format/reset/bootstrap. Boot without a user login and with the build registry unavailable. Test an absent/wrong data mount, unsupported cgroup delegation, exhausted reset spares, empty Access groups, and a user removal followed by Terraform apply. Expect visible failure or denial without unbounded allocation, empty-state initialization, or restored access.
10. **Cloud promotion and handback:** time detection, independent-runner queueing, fencing, VM allocation, prebuilt-image boot, state/dataset load, fresh workspace creation, hosted checks and routing separately. Repeat the ten-user-plus-ingestion acceptance on cloud from Ottawa; reduced capacity does not pass. Prove current grants, exact dataset hashes, consent restrictions and the single spending ledger survive without private-workspace transfer. Simulate unavailable Ubuntu, absent operator laptop, stale/partitioned lease holder, failed reservation-journal sync, missing artifact and provider allocation delay. Terraform apply must not undo promotion. Hand back current control/research/dataset changes without overwriting Ubuntu private files; cloud private work may be discarded as disclosed. Record timing distributions, actual charges, failed attempts and unmet targets; local VM tests alone do not pass.

A successful localhost page or ten open idle tabs is insufficient evidence of ten usable sandboxes. Keep the acceptance report separate from this proposed design.

## Decisions and remaining deployment inputs

- [x] Four admins and three regular users recorded privately; invited enrollment and email PIN login accepted.
- [x] Workspace retention, role/audience boundaries and dataset permissions accepted; no public enrollment.
- [x] Small Canadian government climate sources chosen as a direction; published GeoTIFF `.tiff` and Parquet-only tables required. Concrete AAFC/ECCC candidates are linked; importer outputs remain unvalidated.
- [x] DeepSeek V4.1 Flash/OpenRouter selected; evaluated provider/profile evidence linked; US$100 project allowance pauses new requests until an admin increases it.
- [x] Existing participant consent confirmed by owner; no new participant notice or repeat-consent gate.
- [x] Same-capacity cloud service required, all users in Ottawa, and a maximum 15-minute cold-start target after failure detection accepted. Private workspace transfer is explicitly not required; deployed timing remains unverified.
- [ ] Verify/approve the exact low-cost domain and its registration/renewal checkout; configure the project-owned registrar/Cloudflare account. No purchase yet.
- [ ] Select Canadian-region provider, full-service instance/storage, standing and active monthly ceilings, and authorize timed drills against the accepted 15-minute target. A warm standby is conditional, not assumed purchased.
- [ ] Set advertised demo hours, cohort duration, per-request/per-user guardrails, and operational contacts.
- [ ] Pin initial source objects/checksums, verified station/date/raster subset, output sizes, importer settings, and licenses/attribution; validate real Parquet/GeoTIFF outputs.
- [ ] Confirm research-event retention, export reviewers, Phase 2 ownership, and recovery age for non-workspace research records; these are distinct from already accepted workspace retention and consent.
- [ ] Verify Ubuntu management access, current SSD mapping/free space, and safe service/subordinate identities.
- [ ] Select off-host Terraform state/locking, encrypted recovery storage/keys, an independent runner, and the strongly consistent activation/control/billing journal; test their availability and crash semantics before exposure.

The IaC files, local image, management portal, dataset pipeline, broker/collector integration, and isolation proof can be developed using disposable Ubuntu environments, synthetic fixtures, and fake provider responses before deployment inputs are settled. Live hosted validation, domain/account provisioning, cloud charges/data transfer, email invitations, and exposing the service are subsequent implementation actions. Phase 1 is complete only when the hosted assistance and usage-evidence gates pass; an unassisted terminal alone does not meet the objective. Cloud fallback has its own measured acceptance gate.
