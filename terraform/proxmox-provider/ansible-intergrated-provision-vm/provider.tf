provider "proxmox"{
  # url is the hostname (FQDN if you have one) for the proxmox host you'd like to connect to to issue the commands. my proxmox host is 'prox-1u'. Add /api2/json at the end for the API
  pm_api_url = var.proxmox_params.pm_api_url
  # leave tls_insecure set to true unless you have your proxmox SSL certificate situation fully sorted out (if you do, you will know)
  pm_tls_insecure = var.proxmox_params.pm_tls_insecure
  pm_user = var.proxmox_params.pm_user
  pm_password = var.proxmox_params.pm_password
  pm_debug = var.proxmox_params.pm_debug
}
