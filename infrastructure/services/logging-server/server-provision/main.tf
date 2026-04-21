
# Being in test phase, not need to enable dns yet
terraform {
  required_providers {
    ansible = {
      version = "~> 1.3.0"
      source  = "ansible/ansible"
    }
    # dns = {
    #   source  = "hashicorp/dns"
    #   version = "3.4.1"
    # }
  }
}

# variable "key_secret" {
#   sensitive = true
#   type = string
# }
variable "pm_api_url" {}

variable "pm_user" {}

variable "pm_password" {}

variable "pm_debug" {}

variable "pm_tls_insecure" {}
module "logging-server-provision" {
source = "/Users/truonghoangphuloc/Desktop/home-lab/terraform/proxmox-provider/provision-vm"
proxmox_params = {
  pm_api_url = var.pm_api_url
  pm_user = var.pm_user
  pm_password = var.pm_password
  pm_debug = var.pm_debug
  pm_tls_insecure = var.pm_tls_insecure
  pm_timeout = 600
}
target_node = "geekom-dev"
misc = {
  template = "cloudinit-ubuntu-24.04-template"
}
instances_configurations = {
        "logging-server" = {
        vmid = "231"
        cpu = {
        cores = 2
        }
        memory = {
        amount = 6144
        }
        networking = {
        ip = "172.16.1.216"
        }
        disks = {
          scsi = {
            scsi0 = {
              disk = {
                size = "30G"
              }
            }
          }
        }
        }
    }
}

resource "null_resource" "waiting_instances_ready" {
  # waiting for newly created instances to be ready to run ansible
  depends_on = [ module.logging-server-provision ]
  for_each = module.logging-server-provision.output_map
  provisioner "remote-exec" {
    connection {
      host = each.value
      user = "locthp"
      private_key = file("/Users/truonghoangphuloc/.ssh/id_ed25519")
    }
    inline = ["while [ ! -f /var/lib/cloud/instance/boot-finished ]; do echo 'Waiting for cloud-init...'; sleep 1; done"]
  }
}
resource "ansible_host" "hosts" {
  depends_on = [ null_resource.waiting_instances_ready ]
  for_each = module.logging-server-provision.output_map
  name = each.value
  #groups = [ strcontains(each.key,"k8s-master") ? "k8s-masters":"", strcontains(each.key,"k8s-worker") ? "k8s-workers":""]
  groups = ["all"]
}
resource "null_resource" "running-ansible" {
  depends_on = [ ansible_host.hosts ]
    provisioner "local-exec" {
    command = "ansible-playbook -i inventory.yaml ansible/main.yaml"
  }
}
# Update new A record for the provision server
# resource "dns_a_record_set" "prometheus" {
#   zone = "internal.locthp.com."
#   name = "prometheus.central-monitoring"
#   addresses = [
#     module.logging-server-provision.output_map["monitoring-server"]
#   ]
#   ttl = 300
# }
# resource "dns_a_record_set" "grafana" {
#   zone = "internal.locthp.com."
#   name = "grafana.central-monitoring"
#   addresses = [
#     module.logging-server-provision.output_map["monitoring-server"]
#   ]
#   ttl = 300
# }
# resource "dns_a_record_set" "alertmanager" {
#   zone = "internal.locthp.com."
#   name = "alertmanager.central-monitoring"
#   addresses = [
#     module.logging-server-provision.output_map["monitoring-server"]
#   ]
#   ttl = 300
# }