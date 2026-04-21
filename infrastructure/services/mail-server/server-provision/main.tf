variable "pm_api_url" {}

variable "pm_user" {}

variable "pm_password" {}

variable "pm_debug" {}

variable "pm_tls_insecure" {}
module "mail-server-provision" {
source = "/Users/truonghoangphuloc/Desktop/home-lab/terraform/proxmox-provider/provision-vm"
proxmox_params = {
  pm_api_url = var.pm_api_url
  pm_user = var.pm_user
  pm_password = var.pm_password
  pm_debug = var.pm_debug
  pm_tls_insecure = var.pm_tls_insecure
  pm_timeout = 600
}
target_node = "dell-03"
misc = {
  template = "cloudinit-ubuntu-24.04-template"
}
instances_configurations = {
        "mail.internal.locthp.com" = {
        vmid = "208"
        cpu = {
        cores = 2
        }
        memory = {
        amount = 2048
        }
        networking = {
        ip = "172.16.1.8"
        }
        }
    }
}

