mock_provider "cloudflare" {}
variables {
  account_id  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  bucket_name = "feam-research-fixture"
}
run "private_retained_archive" {
  command = plan
  assert {
    condition     = cloudflare_r2_managed_domain.private.enabled == false && cloudflare_r2_bucket.research.storage_class == "Standard"
    error_message = "Research objects must remain private and use the quoted storage class."
  }
  assert {
    condition     = cloudflare_r2_bucket_lifecycle.retention.rules[0].delete_objects_transition.condition.max_age == 2592000
    error_message = "Owner selected thirty-day research retention."
  }
}
