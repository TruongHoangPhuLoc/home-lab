
terraform {
  required_providers {
    ansible = {
      version = "~> 1.3.0"
      source  = "ansible/ansible"
    }
  }
}

variable "pm_api_url" {}

variable "pm_user" {}

variable "pm_password" {}

variable "pm_debug" {}

variable "pm_tls_insecure" {}
module "monitoring-test-provision" {
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
        "monitoring-test" = {
        vmid = "230"
        cpu = {
        cores = 2
        }
        memory = {
        amount = 4096
        }
        networking = {
        ip = "172.16.1.215"
        }
        }
    }
}

resource "null_resource" "waiting_instances_ready" {
  # waiting for newly created instances to be ready to run ansible
  depends_on = [ module.monitoring-test-provision ]
  for_each = module.monitoring-test-provision.output_map
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
  for_each = module.monitoring-test-provision.output_map
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

