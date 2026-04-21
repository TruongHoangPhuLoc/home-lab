terraform {
  required_providers {
    proxmox = {
      source = "telmate/proxmox"
      version = "3.0.1-rc1"
    }
  }
}

provider "proxmox"{
  # url is the hostname (FQDN if you have one) for the proxmox host you'd like to connect to to issue the commands. my proxmox host is 'prox-1u'. Add /api2/json at the end for the API
  pm_api_url = "https://172.16.1.253:8006/api2/json"
  # leave tls_insecure set to true unless you have your proxmox SSL certificate situation fully sorted out (if you do, you will know)
  pm_tls_insecure = true
  # Change
  #pm_user
  # Change
  #pm_password
  pm_debug = true
}
locals {
  vm_name          = "rockylinux"
  pve_node         = "geekom-dev"
  iso_storage_pool = "local"
}

resource "proxmox_vm_qemu" "provision-proxmox-vms" {
  count       = 1
  name        = "rockylinux-vm"
  desc        = "desc"
  target_node = "geekom-dev"
  
  ### or for a Clone VM operation
  os_type	 = "cloud-init"
  clone = var.template_name
  cpu = "x86-64-v2-AES"
  cores = 1
  sockets = 1
  memory = 2048
  scsihw = "virtio-scsi-single"
  cloudinit_cdrom_storage = "local-lvm"
  agent = 1
  disks {
    scsi {
      scsi0 {
        disk {
          iothread = true
          size     = 25
          storage  = "local-lvm"
        }
      }
    }
  }
  ipconfig0 = "ip=172.16.1.222/24,gw=172.16.1.1"
  nameserver = "172.16.1.5 172.16.1.6"
  searchdomain = "."
  ciuser = "locthp"
  sshkeys = var.ssh_keys
}
