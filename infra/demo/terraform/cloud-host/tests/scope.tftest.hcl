mock_provider "digitalocean" {}
variables {
  deployment_id         = "11111111-1111-4111-8111-111111111111"
  approved_quote_sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  ssh_key_fingerprint   = "fixture"
  operator_cidr         = "192.0.2.1/32"
}
run "disposable_scope" {
  command = plan
  assert {
    condition     = digitalocean_droplet.demo.size == "s-8vcpu-16gb" && digitalocean_volume.service.size == 320 && digitalocean_droplet.demo.backups == false
    error_message = "Quote and full service storage must match; no unapproved backups."
  }
  assert {
    condition     = length(digitalocean_firewall.demo.inbound_rule) == 1 && one(digitalocean_firewall.demo.inbound_rule).port_range == "22"
    error_message = "Only operator SSH may be inbound; no public origin or Docker port."
  }
  assert {
    condition     = digitalocean_volume.service.initial_filesystem_type == null
    error_message = "Provider formatting bypasses the explicit blank-volume capacity check."
  }
  assert {
    condition     = output.service_volume_bootstrap.bytes == 343597383680 && output.service_volume_bootstrap.mount == "/home/feam-service-data" && output.service_volume_bootstrap.filesystem_uuid == var.deployment_id && output.service_volume_bootstrap.approved_quote_sha256 == var.approved_quote_sha256
    error_message = "Bootstrap must bind the approved exact fixed service volume."
  }
}
run "refuse_public_ssh" {
  command = plan
  variables { operator_cidr = "0.0.0.0/0" }
  expect_failures = [var.operator_cidr]
}
