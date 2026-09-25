mock_provider "cloudflare" {}
variables {
  account_id       = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  zone_id          = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  team_name        = "fixture-team"
  user_group_id    = "11111111-1111-4111-8111-111111111111"
  admin_group_id   = "22222222-2222-4222-8222-222222222222"
  email_pin_idp_id = "33333333-3333-4333-8333-333333333333"
  workspace_ids    = ["44444444-4444-4444-8444-444444444444"]
}
run "disabled_by_default" {
  command = plan
  assert {
    condition     = toset(keys(cloudflare_zero_trust_tunnel_cloudflared.site)) == toset(["ubuntu"])
    error_message = "The functional milestone must create only an Ubuntu tunnel."
  }
  assert {
    condition     = length(cloudflare_dns_record.host) == 0
    error_message = "Unreviewed configuration must publish no DNS routes."
  }
  assert {
    condition     = alltrue([for s in cloudflare_zero_trust_tunnel_cloudflared_config.site : alltrue([for i in s.config.ingress : i.service == "http_status:404"])])
    error_message = "Every tunnel must deny traffic until selected."
  }
  assert {
    condition     = length(cloudflare_zero_trust_access_application.host) == 3 && cloudflare_zero_trust_access_application.host["admin.613202690.xyz"].session_duration == "1h"
    error_message = "Use exact sibling hosts and a separate short admin session."
  }
  assert {
    condition     = length(cloudflare_ruleset.https) == 0 && alltrue([for a in cloudflare_zero_trust_access_application.host : length(a.allowed_idps) == 1 && contains(a.allowed_idps, var.email_pin_idp_id) && a.http_only_cookie_attribute && a.same_site_cookie_attribute == "strict" && !a.allow_authenticate_via_warp && !a.options_preflight_bypass && !a.allow_iframe])
    error_message = "Default must not claim zone redirect ownership or relax application admission."
  }
}
run "activation_requires_receipt" {
  command = plan
  variables { selected_site = "ubuntu" }
  expect_failures = [cloudflare_zero_trust_tunnel_cloudflared_config.site]
}
run "one_selected_site" {
  command = plan
  variables {
    selected_site                     = "ubuntu"
    activation_receipt_sha256         = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    manage_https_redirect             = true
    redirect_ownership_receipt_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  }
  assert {
    condition     = length(cloudflare_dns_record.host) == 3 && toset(keys(cloudflare_zero_trust_tunnel_cloudflared_config.site)) == toset(["ubuntu"])
    error_message = "Only Ubuntu is provisioned for the functional milestone."
  }
  assert {
    condition     = length(cloudflare_zero_trust_tunnel_cloudflared_config.site["ubuntu"].config.ingress) == 4 && cloudflare_zero_trust_tunnel_cloudflared_config.site["ubuntu"].config.ingress[3].service == "http_status:404" && alltrue([for i in slice(cloudflare_zero_trust_tunnel_cloudflared_config.site["ubuntu"].config.ingress, 0, 3) : i.service == "unix:/run/feam/services/gateway/browser.sock" && i.origin_request.access.required && i.origin_request.access.team_name == var.team_name && length(i.origin_request.access.aud_tag) == 1])
    error_message = "Selected routes must require per-host JWT verification and retain the catch-all 404."
  }
  assert {
    condition     = cloudflare_ruleset.https[0].rules[0].expression == "(not ssl and http.host in {\"admin.613202690.xyz\" \"feam.613202690.xyz\" \"u-44444444-4444-4444-8444-444444444444.613202690.xyz\"})" && cloudflare_ruleset.https[0].rules[0].action_parameters.from_value.preserve_query_string && cloudflare_ruleset.https[0].rules[0].action_parameters.from_value.status_code == 301
    error_message = "Redirects must match only the exact demo host set and preserve the query."
  }
}
run "routing_requires_https" {
  command = plan
  variables {
    selected_site             = "ubuntu"
    activation_receipt_sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  }
  expect_failures = [cloudflare_zero_trust_tunnel_cloudflared_config.site]
}
run "redirect_requires_exclusive_ownership" {
  command = plan
  variables { manage_https_redirect = true }
  expect_failures = [cloudflare_ruleset.https]
}
run "distinct_groups" {
  command = plan
  variables { admin_group_id = "11111111-1111-4111-8111-111111111111" }
  expect_failures = [cloudflare_zero_trust_tunnel_cloudflared_config.site]
}
run "reject_nested_team_or_workspace" {
  command = plan
  variables {
    team_name     = "https://fixture-team.cloudflareaccess.com"
    workspace_ids = ["*.613202690.xyz"]
  }
  expect_failures = [var.team_name, var.workspace_ids]
}
run "manual_cloud_selection" {
  command = plan
  variables {
    provisioned_sites                 = ["ubuntu", "cloud"]
    selected_site                     = "cloud"
    activation_receipt_sha256         = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    manage_https_redirect             = true
    redirect_ownership_receipt_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  }
  assert {
    condition     = length(cloudflare_dns_record.host) == 3 && alltrue([for i in cloudflare_zero_trust_tunnel_cloudflared_config.site["ubuntu"].config.ingress : i.service == "http_status:404"]) && alltrue([for i in slice(cloudflare_zero_trust_tunnel_cloudflared_config.site["cloud"].config.ingress, 0, 3) : i.service == "unix:/run/feam/services/gateway/browser.sock"])
    error_message = "A manual cloud selection must deny every old Ubuntu ingress route."
  }
}
run "cloud_requires_explicit_scope" {
  command = plan
  variables { selected_site = "cloud" }
  expect_failures = [var.selected_site]
}
run "disabled_retains_redirect_ownership" {
  command = plan
  variables {
    manage_https_redirect             = true
    redirect_ownership_receipt_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  }
  assert {
    condition     = length(cloudflare_dns_record.host) == 0 && length(cloudflare_ruleset.https) == 1 && alltrue([for s in cloudflare_zero_trust_tunnel_cloudflared_config.site : alltrue([for i in s.config.ingress : i.service == "http_status:404"])])
    error_message = "Disabling public routing must retain separately owned HTTPS configuration."
  }
}
