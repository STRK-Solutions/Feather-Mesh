# Full-cohort workspace routes

This records the approved proposal and its original checkpoint. The [operator handoff](ubuntu-operator-handoff-20260925.md) records the completed deployment and live results.

The private YAML roster contains four administrators and seven regular users.
The existing portal, admin route and two staging workspace routes are deployed.
Completing the other five workspace routes requires the following exact saved
Terraform plan, reviewed on 2026-09-25:

- Create five Cloudflare Access applications, each for its exact UUID workspace
  hostname under `613202690.xyz`, using the existing email-PIN provider and user
  policy. Their real audience IDs will be read back after creation.
- Create five proxied CNAME records targeting the existing Ubuntu tunnel.
- Extend that tunnel's Unix-socket ingress from four to nine exact hostnames,
  each with its own required Access audience, retaining the final 404 rule.
- Extend the existing HTTPS redirect's hostname list to those same nine hosts.

Plan SHA-256:
`69257776986490677331aafda6a6f4b7fde8c86591652ec1689f98b7fa296282`.
It contains ten creates and two updates, with no deletes or replacements.
Existing portal/admin/workspace applications, tunnel identity, policies and
unrelated resources are unchanged. Actual roster group reconciliation follows
application and workspace readiness; no email invitations are sent.

The first apply attempt was rejected by automatic approval review before
execution because the full-roster request did not explicitly name this public
edge expansion. The owner then directly approved the five Access applications,
five DNS records, nine-host tunnel and redirect changes, and the exact plan hash
above. The same saved plan applied successfully; no alternate plan was used.

Authoritative Cloudflare API readback verified nine distinct Access audiences,
nine matching proxied CNAME records, nine exact tunnel ingress hostnames with
required Access plus the final 404 rule, and the exact nine-host HTTPS redirect.
The four pre-existing audiences were unchanged. The previous private Terraform
variables were retained, the canonical variables now contain seven workspace
IDs, and a fresh refresh plan reported no changes. The private API readback
receipt has SHA-256
`5a230ccc273c9e36bf5fb9db777996451e4786cce6271c290c5e5d8994c84c14`.
This completes the edge resource expansion; roster group reconciliation and
service/workspace readiness remain separate subsequent checks.
