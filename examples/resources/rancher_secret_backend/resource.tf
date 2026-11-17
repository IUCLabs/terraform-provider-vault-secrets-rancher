resource "rancher_secret_backend" "rancher" {
  path        = "rancher"
  rancher_url = "https://rancher.example.com"
  ca_cert     = file("${path.module}/rancher-ca.pem")

  request_timeout_seconds = 30
}
