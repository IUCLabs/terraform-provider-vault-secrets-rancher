# Terraform Provider for the Vault Rancher Secrets Engine

A Terraform provider that manages the
[vault-plugin-secrets-rancher](https://github.com/IUCLabs/vault-plugin-secrets-rancher)
secrets engine on HashiCorp Vault or OpenBao: mounting and configuring the
engine, seeding auto-rotated service accounts, and defining token-minting roles.

- [Provider documentation](docs/index.md)
- [Secrets engine documentation](https://github.com/IUCLabs/vault-plugin-secrets-rancher)

## Requirements

- Terraform 1.0+ or OpenTofu
- HashiCorp Vault or OpenBao with `vault-plugin-secrets-rancher` registered in
  the plugin catalog (see the
  [plugin requirements](https://github.com/IUCLabs/vault-plugin-secrets-rancher#requirements)
  for tested versions)
- Go 1.25+ to build from source

## Quick Start

Prebuilt provider archives are on the [releases page](https://github.com/IUCLabs/terraform-provider-vault-secrets-rancher/releases).

Resources are prefixed `rancher_`, so declare the provider under the local name
`rancher`. The Vault address and token fall back to `VAULT_ADDR` and
`VAULT_TOKEN`.

```hcl
terraform {
  required_providers {
    rancher = {
      source = "IUCLabs/vault-secrets-rancher"
    }
  }
}

provider "rancher" {}

resource "rancher_secret_backend" "rancher" {
  path        = "rancher"
  rancher_url = "https://rancher.example.com"
  ca_cert     = file("rancher-ca.pem")
}

resource "rancher_secret_backend_service_account" "admin" {
  backend    = rancher_secret_backend.rancher.path
  name       = "admin"
  token      = var.rancher_seed_token
  token_name = "token-abc12"
}

resource "rancher_secret_backend_role" "ci" {
  backend         = rancher_secret_backend.rancher.path
  name            = "ci"
  service_account = rancher_secret_backend_service_account.admin.name
  cluster_name    = "c-m-abc123"
  ttl             = 3600
  max_ttl         = 28800
}
```

Then mint a token with `vault read rancher/creds/ci`.

## Resources

| Resource | Manages |
|----------|---------|
| [`rancher_secret_backend`](docs/resources/secret_backend.md) | The engine mount and its `config` (Rancher URL, CA, TLS). |
| [`rancher_secret_backend_service_account`](docs/resources/secret_backend_service_account.md) | A seeded, auto-rotated Rancher token. |
| [`rancher_secret_backend_role`](docs/resources/secret_backend_role.md) | A role that mints short-lived, lease-bound tokens. |

## Development

```sh
make build    # build the provider binary
make test     # unit tests
make docs     # regenerate docs/ from the schema and examples/
make e2e      # apply real Terraform against Vault, the plugin and a mock Rancher (Docker)
```

`make e2e` builds the plugin from a sibling checkout; set `PLUGIN_REPO` to point
elsewhere.

## License

[MIT](LICENSE)
