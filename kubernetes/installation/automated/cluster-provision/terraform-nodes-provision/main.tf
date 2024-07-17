terraform {
  required_providers {
    ansible = {
      version = "~> 1.3.0"
      source  = "ansible/ansible"
    }
  }
}

module "k8s-master-nodes-provision" {
source = "../../../../../terraform/proxmox-provider/provision-vm"
proxmox_params = {
  "pm_api_url": "https://172.16.1.253:8006/api2/json", 
  # Change
  # pm_user="user"
  # Change
  # pm_password="Mypassword"
  "pm_tls_insecure": true 
}
target_node = "geekom-dev"
instance_configruations = {
  k8s-master-01 = {
    cpu = {
      cores = 2
    }
    memory = {
      amount = 4096
    }
    networking = {
      ip = "172.16.1.113"
    }
  }
  k8s-master-02 = {
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.114"
    }
  }
  k8s-master-03 = {
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
        ip = "172.16.1.115"
    }
  }

}

}
# https://stackoverflow.com/questions/62403030/terraform-wait-till-the-instance-is-reachable
resource "null_resource" "waiting_instances_ready" {
  # waiting for newly created instances to be ready to run ansible
  for_each = toset(module.k8s-master-nodes-provision.instances_ip)
  provisioner "remote-exec" {
    connection {
      host = each.key
      user = "locthp"
      private_key = file("/Users/truonghoangphuloc/.ssh/id_ed25519")
    }
    inline = ["while [ ! -f /var/lib/cloud/instance/boot-finished ]; do echo 'Waiting for cloud-init...'; sleep 1; done"]
  }
}
resource "ansible_host" "host" {
  depends_on = [ null_resource.waiting_instances_ready ]
  for_each = toset(module.k8s-master-nodes-provision.instances_ip)
  name = each.key
  groups = ["k8s-masters"]
}
resource "null_resource" "running-ansible" {
  depends_on = [ null_resource.waiting_instances_ready ]
    provisioner "local-exec" {
    command = "ansible-playbook -i inventory.yaml ../ansible/main.yaml"
  }
}
resource "ansible_group" "group" {
  name     = "all"
  variables = {
    ansible_ssh_common_args = "'-o StrictHostKeyChecking=accept-new'",
    control-plane-endpoint="'172.16.1.230'"
  }
}
# resource "ansible_playbook" "playbook" {
#   for_each = toset(module.k8s-nodes-provision.instances_ip)
#   depends_on = [ null_resource.waiting_instances_ready ]
#   playbook   = "../ansible/main.yaml"
#   name       = each.key
#   extra_vars = {
#     ansible_ssh_common_args = "-o StrictHostKeyChecking=accept-new"
#   }
# }

