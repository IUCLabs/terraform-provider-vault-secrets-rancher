terraform {
  required_providers {
    rancher = {
      source = "git.unkin.net/unkin/ranchervaultsecret"
    }
  }
}

provider "rancher" {
  # address / token fall back to VAULT_ADDR / VAULT_TOKEN.
}

# Mount the Rancher secrets engine and write its connection config.
resource "rancher_secret_backend" "rancher" {
  path        = "rancher"
  rancher_url = "https://rancher.example.com"
  ca_cert     = file("${path.module}/rancher-ca.pem")

  request_timeout_seconds = 30
}
