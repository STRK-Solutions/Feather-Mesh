terraform {
  required_version = "= 1.14.5"
  required_providers {
    cloudflare = { source = "cloudflare/cloudflare", version = "= 5.22.0" }
  }
}
provider "cloudflare" {}

variable "account_id" {
  type = string
  validation {
    condition     = can(regex("^[0-9a-f]{32}$", var.account_id))
    error_message = "Use the recorded Cloudflare account ID."
  }
}
variable "zone_id" {
  type = string
  validation {
    condition     = can(regex("^[0-9a-f]{32}$", var.zone_id))
    error_message = "Use the recorded 613202690.xyz zone ID."
  }
}
variable "team_name" {
  type = string
  validation {
    condition     = can(regex("^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$", var.team_name))
    error_message = "Use the exact team label, without a scheme, path or domain suffix."
  }
}
variable "user_group_id" { type = string }
variable "admin_group_id" { type = string }
variable "email_pin_idp_id" { type = string }
variable "workspace_ids" {
  type = set(string)
  validation {
    condition     = length(var.workspace_ids) <= 10 && alltrue([for id in var.workspace_ids : can(regex("^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$", id))])
    error_message = "At most ten recorded opaque workspace IDs are allowed."
  }
}
variable "selected_site" {
  type    = string
  default = "disabled"
  validation {
    condition     = contains(["disabled", "ubuntu", "cloud"], var.selected_site)
    error_message = "One operator-selected site or disabled is required."
  }
  validation {
    condition     = var.selected_site == "disabled" || contains(var.provisioned_sites, var.selected_site)
    error_message = "The selected site must be explicitly provisioned; cloud is shelved by default."
  }
}
variable "provisioned_sites" {
  type        = set(string)
  default     = ["ubuntu"]
  description = "Ubuntu-only for the functional milestone. Preserve an existing cloud tunnel in state if one was previously provisioned; adding cloud requires resumed scope."
  validation {
    condition     = contains(var.provisioned_sites, "ubuntu") && alltrue([for site in var.provisioned_sites : contains(["ubuntu", "cloud"], site)])
    error_message = "Provision Ubuntu and, only when separately in scope, the optional cloud site."
  }
}
variable "activation_receipt_sha256" {
  type        = string
  default     = ""
  description = "Private reviewed readiness/authorization receipt; disabled until separately approved."
}
variable "manage_https_redirect" {
  type        = bool
  default     = false
  description = "Opt in only after inspecting the zone's Single Redirect phase; see W8.md. Remains enabled when routing is disabled."
}
variable "redirect_ownership_receipt_sha256" {
  type        = string
  default     = ""
  description = "Hash of reviewed inventory proving this phase is absent or exclusively FEAM-owned; never adopt unrelated rules."
}
locals {
  hosts = merge({
    "feam.613202690.xyz"  = "user",
    "admin.613202690.xyz" = "admin"
  }, { for id in var.workspace_ids : "u-${id}.613202690.xyz" => "user" })
}
# Membership is owned exclusively by the reconciler. No group resources,
# roster, exact emails or membership data sources belong in Terraform state.
resource "cloudflare_zero_trust_access_policy" "cohort" {
  for_each         = toset(["user", "admin"])
  account_id       = var.account_id
  name             = "feam-demo-${each.key}"
  decision         = "allow"
  include          = [{ group = { id = each.key == "admin" ? var.admin_group_id : var.user_group_id } }]
  session_duration = each.key == "admin" ? "1h" : "8h"
}
resource "cloudflare_zero_trust_access_application" "host" {
  for_each                    = local.hosts
  account_id                  = var.account_id
  name                        = "feam-demo-${each.key}"
  domain                      = each.key
  type                        = "self_hosted"
  allowed_idps                = [var.email_pin_idp_id]
  auto_redirect_to_identity   = true
  app_launcher_visible        = false
  allow_authenticate_via_warp = false
  allow_iframe                = false
  http_only_cookie_attribute  = true
  same_site_cookie_attribute  = "strict"
  options_preflight_bypass    = false
  session_duration            = each.value == "admin" ? "1h" : "8h"
  policies                    = [{ id = cloudflare_zero_trust_access_policy.cohort[each.value].id, precedence = 1 }]
}
resource "cloudflare_zero_trust_tunnel_cloudflared" "site" {
  for_each   = var.provisioned_sites
  account_id = var.account_id
  name       = "feam-demo-${each.key}"
  config_src = "cloudflare"
}
resource "cloudflare_zero_trust_tunnel_cloudflared_config" "site" {
  for_each   = cloudflare_zero_trust_tunnel_cloudflared.site
  account_id = var.account_id
  tunnel_id  = each.value.id
  config = {
    ingress = concat([for host, role in local.hosts : {
      hostname = host
      service  = var.selected_site == each.key ? "unix:/run/feam/services/gateway/browser.sock" : "http_status:404"
      origin_request = {
        access = { required = true, team_name = var.team_name, aud_tag = [cloudflare_zero_trust_access_application.host[host].aud] }
      }
    }], [{ service = "http_status:404" }])
  }
  lifecycle {
    precondition {
      condition     = var.selected_site == "disabled" || can(regex("^[0-9a-f]{64}$", var.activation_receipt_sha256))
      error_message = "Public routing requires the reviewed readiness receipt and explicit activation approval."
    }
    precondition {
      condition     = var.selected_site == "disabled" || var.manage_https_redirect
      error_message = "Public routing requires the exact-host HTTPS redirect configuration."
    }
    precondition {
      condition     = var.user_group_id != var.admin_group_id
      error_message = "User and admin membership must use distinct recorded Access groups."
    }
  }
}
# The provider manages an entire phase entrypoint, not one independently owned
# rule. Never import an entrypoint with unrelated rules. prevent_destroy keeps
# disabling a deployment from silently deleting durable redirect ownership.
resource "cloudflare_ruleset" "https" {
  count       = var.manage_https_redirect ? 1 : 0
  zone_id     = var.zone_id
  name        = "feam-demo-https"
  description = "Exact FEAM demo hosts only; separately retained edge state"
  kind        = "zone"
  phase       = "http_request_dynamic_redirect"
  rules = [{
    ref         = "feam_demo_https"
    description = "Preserve the exact host, path and query over HTTPS"
    enabled     = true
    # The Rules language exposes the client transport as `ssl`, not a scheme field.
    expression = "(not ssl and http.host in {${join(" ", [for host in sort(keys(local.hosts)) : jsonencode(host)])}})"
    action     = "redirect"
    action_parameters = {
      from_value = {
        status_code           = 301
        preserve_query_string = true
        target_url            = { expression = "concat(\"https://\", http.host, http.request.uri.path)" }
      }
    }
  }]
  lifecycle {
    prevent_destroy = true
    precondition {
      condition     = can(regex("^[0-9a-f]{64}$", var.redirect_ownership_receipt_sha256))
      error_message = "Inspect the zone phase and review its exclusive ownership before enabling this ruleset."
    }
  }
}
resource "cloudflare_dns_record" "host" {
  for_each = var.selected_site == "disabled" || !contains(var.provisioned_sites, var.selected_site) ? {} : local.hosts
  zone_id  = var.zone_id
  name     = each.key
  type     = "CNAME"
  content  = "${cloudflare_zero_trust_tunnel_cloudflared.site[var.selected_site].id}.cfargotunnel.com"
  proxied  = true
  ttl      = 1
  comment  = "FEAM demo; operator selected deployment"
}
output "audiences" { value = { for host, app in cloudflare_zero_trust_access_application.host : host => app.aud } }
output "tunnel_ids" { value = { for site, tunnel in cloudflare_zero_trust_tunnel_cloudflared.site : site => tunnel.id } }
output "issuer" { value = "https://${var.team_name}.cloudflareaccess.com" }
output "selected_site" { value = var.selected_site }
