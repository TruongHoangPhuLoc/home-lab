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
  }
}
variable "key_secret" {
  sensitive = true
  type = string
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

variable "vip" {
  default = "172.16.1.100"
}
module "mysql_cluster_provision" {
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
instances_configurations = {
  mysql-haproxy-01 = {
    vmid = "100"
    cpu = {
      cores = 2
    }
    memory = {
      amount = 2048
    }
    networking = {
      ip = "172.16.1.101"
    }
  }
  mysql-haproxy-02 = {
    vmid = "101"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 2048
    }
    networking = {
      ip = "172.16.1.102"
    }
  }
  mysql-db-01 = {
    vmid = "102"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.103"
    }
    disks = {
        scsi = {
            scsi1 = {
                disk = {
                    size = "50G"
                }
            }
        }
    }
  }
  mysql-db-02 = {
    vmid = "103"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.104"
    }
    disks = {
        scsi = {
            scsi1 = {
                disk = {
                    size = "50G"
                }
            }
        }
    }
  }
  mysql-db-03 = {
    vmid = "104"
    cpu = {
        cores = 2
    }
    memory = {
        amount = 4096
    }
    networking = {
      ip = "172.16.1.105"
    }
    disks = {
        scsi = {
            scsi1 = {
                disk = {
                    size = "50G"
                }
            }
        }
    }
  }
}
misc = {
  template = "cloudinit-ubuntu-22.04-template"
}
}
resource "null_resource" "waiting_instances_ready" {
  # waiting for newly created instances to be ready to run ansible
  depends_on = [ module.mysql_cluster_provision ]
  for_each = module.mysql_cluster_provision.output_map
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
  for_each = module.mysql_cluster_provision.output_map
  name = each.value
  groups = [ coalesce(strcontains(each.key,"mysql-db") ? "db-hosts":"", strcontains(each.key,"haproxy-01") ? "haproxy-master":"", ! strcontains(each.key,"haproxy-01") ? "haproxy-slave":""), strcontains(each.key,"haproxy") ? "haproxy":"all"]
}


resource "null_resource" "running-ansible" {
  depends_on = [ ansible_host.hosts ]
    provisioner "local-exec" {
    command = "ansible-playbook -i inventory.yaml ./ansible/main.yaml"
  }
}
resource "ansible_group" "group-all" {
  name     = "all"
  variables = {
    VIP=var.vip,
  }
}

resource "dns_a_record_set" "mysql_dns_record" {
  zone = "internal.locthp.com."
  name = "mysql"
  addresses = [
    var.vip
  ]
  ttl = 300
}