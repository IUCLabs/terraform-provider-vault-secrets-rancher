# Import by mount path. ca_cert is not readable from the engine, so set it in
# configuration after import.
terraform import rancher_secret_backend.rancher rancher
