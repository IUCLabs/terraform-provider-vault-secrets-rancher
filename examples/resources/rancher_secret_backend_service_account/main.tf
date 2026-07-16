# Seed a Rancher token the engine will auto-rotate. Rancher caps token TTLs
# (commonly 90d); the engine rotates at rotation_period (default 45d) by minting
# a fresh token with the current one, so the credential never lapses.
resource "rancher_secret_backend_service_account" "admin" {
  backend = rancher_secret_backend.rancher.path
  name    = "admin"

  # The initial token. Write-only: the engine rotates it from here on.
  token = var.rancher_seed_token

  # metadata.name of the seed token, so the first rotation can delete it.
  token_name = var.rancher_seed_token_name

  token_ttl       = 90 * 24 * 3600 # 90d
  rotation_period = 45 * 24 * 3600 # 45d
}

variable "rancher_seed_token" {
  type      = string
  sensitive = true
}

variable "rancher_seed_token_name" {
  type    = string
  default = ""
}
