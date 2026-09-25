terraform {
  required_version = "= 1.14.5"
  required_providers {
    digitalocean = { source = "digitalocean/digitalocean", version = "= 2.102.0" }
  }
}
provider "digitalocean" {}
variable "deployment_id" {
  type = string
  validation {
    condition     = can(regex("^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$", var.deployment_id))
    error_message = "Explicit fresh deployment UUID required."
  }
}
variable "approved_quote_sha256" {
  type = string
  validation {
    condition     = can(regex("^[0-9a-f]{64}$", var.approved_quote_sha256))
    error_message = "Use only after the exact quote and bounded test are approved."
  }
}
variable "ssh_key_fingerprint" { type = string }
variable "operator_cidr" {
  type = string
  validation {
    condition     = can(cidrhost(var.operator_cidr, 0)) && !contains(["0.0.0.0/0", "::/0"], var.operator_cidr)
    error_message = "SSH must be restricted to the operator network."
  }
}
locals { name = "feam-${var.deployment_id}" }
resource "digitalocean_droplet" "demo" {
  name       = local.name
  region     = "nyc3"
  size       = "s-8vcpu-16gb"
  image      = "ubuntu-24-04-x64"
  ssh_keys   = [var.ssh_key_fingerprint]
  backups    = false
  monitoring = true
  tags       = ["feam-demo", local.name]
}
resource "digitalocean_volume" "service" {
  name   = "${local.name}-service"
  region = "nyc3"
  size   = 320
  # Leave blank: cloud_volume.py explicitly initializes only the reviewed new
  # device, using an inode density/reserve that preserves the full pool floor.
  description = "Disposable FEAM service data; research and project spending are external"
}
resource "digitalocean_volume_attachment" "service" {
  droplet_id = digitalocean_droplet.demo.id
  volume_id  = digitalocean_volume.service.id
}
resource "digitalocean_firewall" "demo" {
  name        = local.name
  droplet_ids = [digitalocean_droplet.demo.id]
  inbound_rule {
    protocol         = "tcp"
    port_range       = "22"
    source_addresses = [var.operator_cidr]
  }
  outbound_rule {
    protocol              = "tcp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
  outbound_rule {
    protocol              = "udp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
  outbound_rule {
    protocol              = "icmp"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
}
output "host" {
  value     = digitalocean_droplet.demo.ipv4_address
  sensitive = true
}
output "volume_id" { value = digitalocean_volume.service.id }
output "deployment_id" { value = var.deployment_id }
output "service_volume_bootstrap" {
  value = {
    protocol              = "feam.cloud-volume.v1"
    deployment_id         = var.deployment_id
    volume_id             = digitalocean_volume.service.id
    droplet_id            = tostring(digitalocean_droplet.demo.id)
    approved_quote_sha256 = var.approved_quote_sha256
    region                = "nyc3"
    volume_name           = digitalocean_volume.service.name
    bytes                 = 343597383680
    filesystem_uuid       = var.deployment_id
    mount                 = "/home/feam-service-data"
  }
}
