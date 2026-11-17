# Import by <backend>/service-accounts/<name>. The engine never returns the
# token, so it must still be set in configuration.
terraform import rancher_secret_backend_service_account.admin rancher/service-accounts/admin
