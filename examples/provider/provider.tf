terraform {
  required_providers {
    # Resources are prefixed rancher_, so declare the provider under the local name "rancher".
    rancher = {
      source = "IUCLabs/vault-secrets-rancher"
    }
  }
}

# address and token fall back to VAULT_ADDR and VAULT_TOKEN.
provider "rancher" {
  address = "https://vault.example.com:8200"
}
