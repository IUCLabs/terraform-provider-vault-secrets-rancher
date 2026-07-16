# terraform-provider-ranchervaultsecret

Terraform provider for the [vault-plugin-secrets-rancher](https://git.unkin.net/unkin/vault-plugin-secrets-rancher)
secrets engine. It manages the engine's configuration on HashiCorp Vault or
OpenBao — mounting it, seeding auto-rotated service-account tokens, and defining
token-minting roles.

The provider's source address is `git.unkin.net/unkin/ranchervaultsecret`; its
resources are prefixed `rancher_` (declare it under the local name `rancher` in
`required_providers`).

## Resources

| Resource | Manages |
|----------|---------|
| `rancher_secret_backend` | Mounts the engine and writes `config` (Rancher URL, CA, TLS). |
| `rancher_secret_backend_service_account` | A seeded, auto-rotated Rancher token (`token`, `token_ttl`, `rotation_period`). |
| `rancher_secret_backend_role` | A minting role (`service_account`, `cluster_name`, `ttl`, `max_ttl`, `description`). |

## Example

```hcl
provider "rancher" {} # address/token from VAULT_ADDR / VAULT_TOKEN

resource "rancher_secret_backend" "rancher" {
  path        = "rancher"
  rancher_url = "https://rancher.example.com"
  ca_cert     = file("rancher-ca.pem")
}

resource "rancher_secret_backend_service_account" "admin" {
  backend         = rancher_secret_backend.rancher.path
  name            = "admin"
  token           = var.rancher_seed_token
  token_ttl       = 90 * 24 * 3600
  rotation_period = 45 * 24 * 3600
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

Then mint a token: `vault read rancher/creds/ci`.

## Development

```sh
make build      # build the provider binary
make install    # install into ~/.terraform.d/plugins for local dev_overrides
make test       # unit tests
make e2e        # apply real terraform against Vault + the plugin + mock Rancher
make package    # zip for the artifactapi terraform registry
```

Releases are tag-driven (`make patch|minor|major`): a Woodpecker pipeline zips
the provider and uploads it to the internal artifactapi terraform registry,
installable via `source = "git.unkin.net/unkin/ranchervaultsecret"`.
