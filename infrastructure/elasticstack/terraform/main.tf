provider "dns" {
  update {
    server        = "172.16.1.2"
    key_name      = "tsig-key."
    key_algorithm = "hmac-sha256"
    key_secret    = var.key_secret
  }
}
terraform {
  required_providers {
    ansible = {
      version = "~> 1.3.0"
      source  = "ansible/ansible"
    }
    dns = {
      source  = "hashicorp/dns"
      version = "3.4.1"
    }
  }
}

variable "key_secret" {
  sensitive = true
  type = string
}
variable "pm_api_url" {}

variable "pm_user" {}

variable "pm_password" {}

variable "pm_debug" {}

variable "pm_tls_insecure" {}
module "elasticstack-server-provision" {
source = "/Users/truonghoangphuloc/Desktop/home-lab/terraform/proxmox-provider/provision-vm"
proxmox_params = {
  pm_api_url = var.pm_api_url
  pm_user = var.pm_user
  pm_password = var.pm_password
  pm_debug = var.pm_debug
  pm_tls_insecure = var.pm_tls_insecure
  pm_timeout = 600
}
target_node = "intelnuc-01"
misc = {
  template = "cloudinit-ubuntu-24.04-template"
}
instances_configurations = {
        "elasticstack-server" = {
        vmid = "106"
        cpu = {
        cores = 4
        }
        memory = {
        amount = 8192
        }
        networking = {
        ip = "172.16.1.106"
        }
        disks = {
            scsi = {
                scsi0 = {
                    disk = {
                        size = "50G"
                    }
                }
            }
        }
        }
    }
}

resource "null_resource" "waiting_instances_ready" {
  # waiting for newly created instances to be ready to run ansible
  depends_on = [ module.elasticstack-server-provision ]
  for_each = module.elasticstack-server-provision.output_map
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
  for_each = module.elasticstack-server-provision.output_map
  name = each.value
  #groups = [ strcontains(each.key,"k8s-master") ? "k8s-masters":"", strcontains(each.key,"k8s-worker") ? "k8s-workers":""]
  groups = ["all"]
}
resource "ansible_group" "group-all" {
  name     = "all"
  variables = {
    NODE_EXPORTER_YML_URL="https://raw.githubusercontent.com/TruongHoangPhuLoc/home-lab/main/compose_files/node-exporter-docker-compose.yml"
    PROMTAIL_YML_URL="https://raw.githubusercontent.com/TruongHoangPhuLoc/home-lab/main/compose_files/promtail-docker-compose.yml"
  }
}
resource "null_resource" "running-ansible" {
  depends_on = [ ansible_host.hosts ]
    provisioner "local-exec" {
    command = "ansible-playbook -i inventory.yaml /Users/truonghoangphuloc/Desktop/home-lab/ansible-agents-installation/main.yaml"
  }
}
# Update new A record for the provision server
resource "dns_a_record_set" "kibana" {
  zone = "internal.locthp.com."
  name = "kibana.central-monitoring"
  addresses = [
    module.elasticstack-server-provision.output_map["elasticstack-server"]
  ]
  ttl = 604800
}
resource "dns_a_record_set" "elasticsearch" {
  zone = "internal.locthp.com."
  name = "elasticsearch.central-monitoring"
  addresses = [
    module.elasticstack-server-provision.output_map["elasticstack-server"]
  ]
  ttl = 604800
}

