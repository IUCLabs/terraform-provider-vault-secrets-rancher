terraform {
  required_providers {
    rancher = {
      source = "git.unkin.net/unkin/ranchervaultsecret"
    }
  }
}

provider "rancher" {
  address = "http://127.0.0.1:8200"
  token   = "root"
}

resource "rancher_secret_backend" "rancher" {
  path        = "rancher"
  description = "Rancher token engine (e2e)"
  # Reachable from inside the vault container, where the plugin runs.
  rancher_url     = "http://rancher:8443"
  tls_skip_verify = true
}

resource "rancher_secret_backend_service_account" "admin" {
  backend = rancher_secret_backend.rancher.path
  name    = "admin"

  token      = "seed-token"
  token_name = "seed"

  token_ttl       = 90 * 24 * 3600
  rotation_period = 45 * 24 * 3600
}

resource "rancher_secret_backend_role" "ci" {
  backend         = rancher_secret_backend.rancher.path
  name            = "ci"
  service_account = rancher_secret_backend_service_account.admin.name
  cluster_name    = "c-m-abc123"
  ttl             = 3600
  max_ttl         = 86400
}
