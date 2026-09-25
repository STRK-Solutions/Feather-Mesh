terraform {
  required_version = "= 1.14.5"
  required_providers {
    cloudflare = { source = "cloudflare/cloudflare", version = "= 5.22.0" }
  }
}
provider "cloudflare" {}
variable "account_id" { type = string }
variable "bucket_name" {
  type = string
  validation {
    condition     = can(regex("^feam-research-[a-z0-9-]+$", var.bucket_name))
    error_message = "A dedicated FEAM research bucket is required."
  }
}
resource "cloudflare_r2_bucket" "research" {
  account_id    = var.account_id
  name          = var.bucket_name
  location      = "ENAM"
  storage_class = "Standard"
  lifecycle { prevent_destroy = true }
}
resource "cloudflare_r2_managed_domain" "private" {
  account_id  = var.account_id
  bucket_name = cloudflare_r2_bucket.research.name
  enabled     = false
}
# Owner selected 30-day retention for research events and exports. The
# collector enforces event-time expiry as well; upload time is not an extension.
resource "cloudflare_r2_bucket_lifecycle" "retention" {
  account_id  = var.account_id
  bucket_name = cloudflare_r2_bucket.research.name
  rules = [{
    id                        = "research-retention-30-days"
    enabled                   = true
    conditions                = { prefix = "research/" }
    delete_objects_transition = { condition = { type = "Age", max_age = 2592000 } }
    }, {
    id                                 = "abort-incomplete-uploads-1-day"
    enabled                            = true
    conditions                         = { prefix = "" }
    abort_multipart_uploads_transition = { condition = { type = "Age", max_age = 86400 } }
  }]
}
output "bucket_name" { value = cloudflare_r2_bucket.research.name }
