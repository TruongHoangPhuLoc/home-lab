

resource "proxmox_vm_qemu" "provision-proxmox-vm" {
  name = var.instance_configruations.name
  desc        = ""
  target_node = var.target_node
  ### or for a Clone VM operation
  os_type	 = "cloud-init"
  clone = var.misc.template
  cpu = var.instance_configruations.cpu.type
  cores = var.instance_configruations.cpu.cores
  sockets = var.instance_configruations.cpu.sockets
  memory = var.instance_configruations.memory.amount
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
        for_each = var.instance_configruations.disks.scsi.scsi0.disk.size != null ? [0] : []
        content {
          dynamic "disk" {
            for_each = var.instance_configruations.disks.scsi.scsi0.disk.size != null ? [0] : []
            content {
              backup     = var.instance_configruations.disks.scsi.scsi0.disk.backup
              cache      = var.instance_configruations.disks.scsi.scsi0.disk.cache
              emulatessd = var.instance_configruations.disks.scsi.scsi0.disk.emulatessd
              format     = var.instance_configruations.disks.scsi.scsi0.disk.format
              iothread   = var.instance_configruations.disks.scsi.scsi0.disk.iothread
              replicate  = var.instance_configruations.disks.scsi.scsi0.disk.replicate
              size       = var.instance_configruations.disks.scsi.scsi0.disk.size
              storage    = var.instance_configruations.disks.scsi.scsi0.disk.storage
            }
          }
        }
      }
      # disk1 (optional)
      dynamic "scsi1" {
        for_each = var.instance_configruations.disks.scsi.scsi1.disk.size != null ? [0] : []
        content {
          dynamic "disk" {
            for_each = var.instance_configruations.disks.scsi.scsi1.disk.size != null ? [0] : []
            content {
              backup     = var.instance_configruations.disks.scsi.scsi1.disk.backup
              cache      = var.instance_configruations.disks.scsi.scsi1.disk.cache
              emulatessd = var.instance_configruations.disks.scsi.scsi1.disk.emulatessd
              format     = var.instance_configruations.disks.scsi.scsi1.disk.format
              iothread   = var.instance_configruations.disks.scsi.scsi1.disk.iothread
              replicate  = var.instance_configruations.disks.scsi.scsi1.disk.replicate
              size       = var.instance_configruations.disks.scsi.scsi1.disk.size
              storage    = var.instance_configruations.disks.scsi.scsi1.disk.storage
            }
          }
        }
      }
      # disk2 (optional)
      dynamic "scsi2" {
        for_each = var.instance_configruations.disks.scsi.scsi2.disk.size != null ? [0] : []
        content {
          dynamic "disk" {
            for_each = var.instance_configruations.disks.scsi.scsi2.disk.size != null ? [0] : []
            content {
              backup     = var.instance_configruations.disks.scsi.scsi2.disk.backup
              cache      = var.instance_configruations.disks.scsi.scsi2.disk.cache
              emulatessd = var.instance_configruations.disks.scsi.scsi2.disk.emulatessd
              format     = var.instance_configruations.disks.scsi.scsi2.disk.format
              iothread   = var.instance_configruations.disks.scsi.scsi2.disk.iothread
              replicate  = var.instance_configruations.disks.scsi.scsi2.disk.replicate
              size       = var.instance_configruations.disks.scsi.scsi2.disk.size
              storage    = var.instance_configruations.disks.scsi.scsi2.disk.storage
            }
          }
        }
      }
    }
  }
  agent = 1
  ipconfig0 = "ip=${var.instance_configruations.networking.ip}/${var.instance_configruations.networking.subnet}, gw=${var.instance_configruations.networking.gateway}"
  nameserver = var.instance_configruations.networking.nameservers
  searchdomain = var.instance_configruations.networking.searchdomain
  ciuser = var.misc.template
  sshkeys = var.ssh_keys
}
