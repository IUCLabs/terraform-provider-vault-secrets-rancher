# A role that mints short-lived tokens scoped to one downstream cluster. Reading
# rancher/creds/ci returns a lease-bound token deleted from Rancher on revoke.
#
# Note: Rancher tokens inherit the service account user's RBAC; only cluster and
# TTL are scoped per-token. For least-privilege, point roles at a service account
# seeded with a purpose-built, limited Rancher user.
resource "rancher_secret_backend_role" "ci" {
  backend         = rancher_secret_backend.rancher.path
  service_account = rancher_secret_backend_service_account.admin.name

  name         = "ci"
  cluster_name = "c-m-abc123"
  description  = "CI/CD ephemeral token"
  ttl          = 3600  # 1h
  max_ttl      = 28800 # 8h
}
