terraform {
  required_providers {
    proxmox = {
      source = "telmate/proxmox"
      version = "3.0.1-rc3"
    }
  }
}

resource "proxmox_vm_qemu" "provision-proxmox-vm" {
  name        = var.vm_name
  desc        = "desc"
  target_node = var.pve_node
  for_each = var.proxmox_vm_qemu_disk
  ### or for a Clone VM operation
  os_type	 = "cloud-init"
  clone = var.template_name
  cpu = var.proxmox_vm_qemu_cpu.type
  cores = var.proxmox_vm_qemu_cpu.cores
  sockets = var.proxmox_vm_qemu_cpu.sockets
  memory = var.proxmox_vm_qemu_memory.amount
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
        for_each = var.proxmox_vm_qemu_disk.value.disks.scsi.scsi0.disk.size != null ? [0] : []
        content {
          dynamic "disk" {
            for_each = var.proxmox_vm_qemu_disk["server1"].disks.scsi.scsi0.disk.size != null ? [0] : []
            content {
              backup     = var.proxmox_vm_qemu_disk["server1"].disks.scsi.scsi0.disk.backup
              cache      = var.proxmox_vm_qemu_disk["server1"].disks.scsi.scsi0.disk.cache
              emulatessd = var.proxmox_vm_qemu_disk["server1"].disks.scsi.scsi0.disk.emulatessd
              format     = var.proxmox_vm_qemu_disk["server1"].disks.scsi.scsi0.disk.format
              iothread   = var.proxmox_vm_qemu_disk["server1"].disks.scsi.scsi0.disk.iothread
              replicate  = var.proxmox_vm_qemu_disk["server1"].disks.scsi.scsi0.disk.replicate
              size       = var.proxmox_vm_qemu_disk["server1"].disks.scsi.scsi0.disk.size
              storage    = var.proxmox_vm_qemu_disk["server1"].disks.scsi.scsi0.disk.storage
            }
          }
        }
      }
    }
  }
  agent = 1
#   ipconfig0 = var.proxmox_vm_qemu_networking.ipconfig0
#   nameserver = var.proxmox_vm_qemu_networking.nameservers
#   searchdomain = var.proxmox_vm_qemu_networking.searchdomain
  ipconfig0 = "ip=${var.proxmox_vm_qemu_networking.ipconfig0}/${var.proxmox_vm_qemu_networking.subnet}, gw=${var.proxmox_vm_qemu_networking.gw}"
  nameserver = var.proxmox_vm_qemu_networking.nameservers
  searchdomain = var.proxmox_vm_qemu_networking.searchdomain
  ciuser = var.ciuser
  sshkeys = var.ssh_keys
}
