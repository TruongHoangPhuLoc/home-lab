terraform {
  required_providers {
    ansible = {
      version = "~> 1.3.0"
      source  = "ansible/ansible"
    }
  }
}
variable "address_pool" {
  type = string
  default = "172.16.3.0/24"
}
variable "clustername" {
  type = string
  default = "terraform-provisioned-cluster"
}
variable "pm_api_url" {
  
}

variable "pm_user" {
  
}

variable "pm_password" {
  
}

variable "pm_debug" {
  
}

variable "pm_tls_insecure" {
  
}
module "k8s-nodes-provision" {
source = "../../../../../terraform/proxmox-provider/provision-vm"
proxmox_params = {
  pm_api_url = var.pm_api_url
  pm_user = var.pm_user
  pm_password = var.pm_password
  pm_debug = var.pm_debug
  pm_tls_insecure = var.pm_tls_insecure
  pm_timeout = 1200
}
target_node = "geekom-dev"
instances_configurations = {
  terraform-haproxy-01 = {
    vmid = "220"
    cpu = {
      cores = 2
    }
    memory = {
      amount = 4096
    }
    networking = {
      ip = "172.16.1.190"
    }
  }
  terraform-haproxy-02 = {
    vmid = "221"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.191"
    }
  }
  # terraform-haproxy-03 = {
  #   vmid = "212"
  #   cpu = {
  #       cores = 2
  #   }
  #   memory = {
  #       amount = 4096
  #   }
  #   networking = {
  #     ip = "172.16.1.192"
  #   }
  # }
  k8s-master-01 = {
    vmid = "200"
    cpu = {
      cores = 2
    }
    memory = {
      amount = 4096
    }
    networking = {
      ip = "172.16.1.231"
    }
  }
  k8s-master-02 = {
    vmid = "201"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.232"
    }
  }
  k8s-master-03 = {
    vmid = "202"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
        ip = "172.16.1.233"
    }
  }
  k8s-worker-01 = {
    vmid = "203"
    cpu = {
      cores = 2
    }
    memory = {
      amount = 4096
    }
    networking = {
      ip = "172.16.1.234"
    }
  }
  k8s-worker-02 = {
    vmid = "204"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.235"
    }
  }
  k8s-worker-03 = {
    vmid = "205"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.236"
    }
  }
  k8s-worker-additional-01 = {
    vmid = "206"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.150"
    }
  }
  k8s-worker-additional-02 = {
    vmid = "207"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.151"
    }
  }
  k8s-worker-additional-03 = {
    vmid = "208"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.152"
    }
  }
  k8s-worker-additional-04 = {
    vmid = "209"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.153"
    }
  }
  k8s-worker-additional-05 = {
    vmid = "210"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.154"
    }
  }
  k8s-worker-additional-06 = {
    vmid = "211"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.155"
    }
  }
  external-bgp-router = {
    vmid = "215"
    cpu = {
      cores = 2
    }
    memory = {
      amount = 2048
    }
    networking = {
      ip = "172.16.1.170"
    }
  }
}
misc = {
  template = "cloudinit-ubuntu-24.04-template"
}
}
# https://stackoverflow.com/questions/62403030/terraform-wait-till-the-instance-is-reachable
resource "null_resource" "waiting_instances_ready" {
  # waiting for newly created instances to be ready to run ansible
  depends_on = [ module.k8s-nodes-provision ]
  for_each = module.k8s-nodes-provision.output_map
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
  for_each = module.k8s-nodes-provision.output_map
  name = each.value
  #groups = [ strcontains(each.key,"k8s-master") ? "k8s-masters":"", strcontains(each.key,"k8s-worker") ? "k8s-workers":""]
  groups = [ coalesce(strcontains(each.key,"k8s-master") ? "k8s-masters":"", strcontains(each.key,"k8s-worker") ? "k8s-workers":"", strcontains(each.key,"haproxy-01") ? "haproxy-master":"", ! strcontains(each.key,"haproxy-01") ? "haproxy-slave":""), strcontains(each.key,"haproxy") ? "haproxy":"all", strcontains(each.key,"external-bgp-router") ? "external-bgp-router":"all"]
}
resource "null_resource" "running-ansible" {
  depends_on = [ ansible_host.hosts ]
    provisioner "local-exec" {
    command = "ansible-playbook -i inventory.yaml ../ansible/main.yaml"
  }
}
resource "ansible_group" "group-all" {
  name     = "all"
  variables = {
    ansible_ssh_common_args = "-o StrictHostKeyChecking=accept-new",
    control_plane_endpoint="172.16.1.180",
    cluster_name=var.clustername
  }
}
resource "ansible_group" "group-external-bgp-router" {
  name     = "external-bgp-router"
  variables = {
    address_pool = var.address_pool
  }
}