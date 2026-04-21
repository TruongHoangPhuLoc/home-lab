provider "dns" {
  update {
    server        = "172.16.1.2"
    key_name      = "tsig-key."
    key_algorithm = "hmac-sha256"
    key_secret    = var.key_secret
  }
}