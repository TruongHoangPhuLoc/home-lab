terraform {
  required_providers {
    proxmox = {
      source = "telmate/proxmox"
      version = "3.0.1-rc3"
    }
    ansible = {
      version = "~> 1.3.0"
      source  = "ansible/ansible"
    }
  }
}

resource "proxmox_vm_qemu" "provision-proxmox-vm" {
  for_each    = var.instance_configruations
  name        = "${each.key}"
  desc        = ""
  target_node = var.target_node
  ### or for a Clone VM operation
  os_type	 = "cloud-init"
  clone = var.misc.template
  cpu = each.value.cpu.type
  cores = each.value.cpu.cores
  sockets = each.value.cpu.sockets
  memory = each.value.memory.amount
  scsihw = "virtio-scsi-pci"
  disks {
    ide {
        ide0 {
            cloudinit {
                storage = "local-lvm"
            }
        }
    }
    scsi {
      # disk0 (system disk)
      dynamic "scsi0" {
        for_each = each.value.disks.scsi.scsi0.disk.size != null ? [0] : []
        content {
          dynamic "disk" {
            for_each = each.value.disks.scsi.scsi0.disk.size != null ? [0] : []
            content {
              backup     = each.value.disks.scsi.scsi0.disk.backup
              cache      = each.value.disks.scsi.scsi0.disk.cache
              emulatessd = each.value.disks.scsi.scsi0.disk.emulatessd
              format     = each.value.disks.scsi.scsi0.disk.format
              iothread   = each.value.disks.scsi.scsi0.disk.iothread
              replicate  = each.value.disks.scsi.scsi0.disk.replicate
              size       = each.value.disks.scsi.scsi0.disk.size
              storage    = each.value.disks.scsi.scsi0.disk.storage
            }
          }
        }
      }
      # disk1 (optional)
      dynamic "scsi1" {
        for_each = each.value.disks.scsi.scsi1.disk.size != null ? [0] : []
        content {
          dynamic "disk" {
            for_each = each.value.disks.scsi.scsi1.disk.size != null ? [0] : []
            content {
              backup     = each.value.disks.scsi.scsi1.disk.backup
              cache      = each.value.disks.scsi.scsi1.disk.cache
              emulatessd = each.value.disks.scsi.scsi1.disk.emulatessd
              format     = each.value.disks.scsi.scsi1.disk.format
              iothread   = each.value.disks.scsi.scsi1.disk.iothread
              replicate  = each.value.disks.scsi.scsi1.disk.replicate
              size       = each.value.disks.scsi.scsi1.disk.size
              storage    = each.value.disks.scsi.scsi1.disk.storage
            }
          }
        }
      }
      # disk2 (optional)
      dynamic "scsi2" {
        for_each = each.value.disks.scsi.scsi2.disk.size != null ? [0] : []
        content {
          dynamic "disk" {
            for_each = each.value.disks.scsi.scsi2.disk.size != null ? [0] : []
            content {
              backup     = each.value.disks.scsi.scsi2.disk.backup
              cache      = each.value.disks.scsi.scsi2.disk.cache
              emulatessd = each.value.disks.scsi.scsi2.disk.emulatessd
              format     = each.value.disks.scsi.scsi2.disk.format
              iothread   = each.value.disks.scsi.scsi2.disk.iothread
              replicate  = each.value.disks.scsi.scsi2.disk.replicate
              size       = each.value.disks.scsi.scsi2.disk.size
              storage    = each.value.disks.scsi.scsi2.disk.storage
            }
          }
        }
      }
    }
  }
  agent = 1
  #ipconfig0 = "ip=${var.proxmox_vm_qemu_networking.ipconfig0}/${var.proxmox_vm_qemu_networking.subnet}, gw=${var.proxmox_vm_qemu_networking.gw}"
  ipconfig0 = "ip=${each.value.networking.ip}/${each.value.networking.subnet},gw=${each.value.networking.gateway}"
  nameserver = "${each.value.networking.nameservers}"
  searchdomain = "${each.value.networking.searchdomain}"
  ciuser = var.misc.ciuser
  sshkeys = var.ssh_keys
}


resource "ansible_host" "host" {
  for_each = var.instance_configruations
  name = each.value.networking.ip
}
resource "ansible_group" "group" {
  name     = "all"
  variables = {
    ansible_ssh_common_args = "'-o StrictHostKeyChecking=accept-new'"
  }
}