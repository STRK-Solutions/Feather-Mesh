# Separate infrastructure lifecycles

These modules are implementation, not deployed acceptance. Terraform 1.14.5,
Cloudflare 5.22.0 and DigitalOcean 2.102.0 are locked. Each directory has its
own restricted operator state. Never combine the three into a disposable
cloud-host state or commit real tfvars/plans/state.

- `edge/`: dedicated Ubuntu Tunnel, per-host Access applications,
  reusable group-referencing policies and exact demo DNS. The default disables
  DNS and denies every tunnel route. Mutable group membership belongs only to
  the reconciler. No zone, registration, unrelated DNS, roster or API token
  resource is declared. Inspect/import existing demo resources first.
  `provisioned_sites` defaults to `["ubuntu"]`; the functional milestone creates
  no cloud tunnel. A previously provisioned cloud tunnel must remain explicitly
  listed to preserve it while shelved. Cloud selection requires an explicit
  scope change and inclusion in that set.
- `research-storage/`: private R2 Standard bucket with public `r2.dev` disabled,
  30-day `research/` expiry and `prevent_destroy`. Its separate state survives
  compute teardown. Collector expiry must use event time, not upload time.
- `cloud-host/`: proposed NYC3 8-vCPU/16-GiB Ubuntu VM plus separate 320-GiB
  service volume, explicit deployment tags and operator-only inbound SSH.
  No public web/Docker port, backup, reserved IP or research bucket is created.
  The recipe still needs cloud account availability and a reviewed live plan.

For each module, run:

```bash
terraform init -backend=false -input=false
terraform fmt -check
terraform validate
terraform test
```

The tests use mock providers and require no credentials. They cover disabled
routing, one selected site, receipt requirements, private archive retention,
fixed cloud quota and denial of unrestricted SSH. They do not prove real
Cloudflare permissions, Rules-language field validity, certificate trust,
mutable membership or deployment. Real API application and browser checks
remain necessary; mocked expression equality does not validate a Cloudflare rule.

For a real plan, move state to owner-only encrypted operator storage, supply
credential environment variables from a private secret file, inspect the
existing resources, and save the plan privately. A nonempty receipt hash is
an operator input, not evidence that readiness passed. Publish no route until
the actual gateway/identity/budget/archive gates and owner approval exist.
Never request `destroy` in the durable edge or research-storage directory as
part of disposable host teardown.

The current connection can manage the selected Cloudflare zone and R2. The
approved private research bucket was created through that connector; import
its reviewed identity into this separate state before any Terraform apply.
Terraform and the reconciler still need their own scoped credentials, as does
the collector's S3 client. The connector cannot export its credential to services
or create account API tokens with its current permissions.
The supplied DigitalOcean token works, but the account currently does not expose
the approved 8-vCPU/16-GiB size. Do not substitute smaller capacity for W9.

Provider documentation: [Cloudflare](https://registry.terraform.io/providers/cloudflare/cloudflare/5.22.0/docs),
[DigitalOcean](https://registry.terraform.io/providers/digitalocean/digitalocean/2.102.0/docs).
