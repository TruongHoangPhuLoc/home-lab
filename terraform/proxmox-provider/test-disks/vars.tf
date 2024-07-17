variable "ssh_keys" {
  default =  <<EOT
  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHgKmDqIR8VZ+sMoCxjt8HTlerwO29A7MS4lQMNehsr3 root@tasks-automation-server
  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIA61Dt7OxM8Jpoy/I0/FmCLjaqjNApU+UO+vRpyavBoj truonghoangphuloc@phus-MacBook-Pro.local
  EOT
}
variable "vm_name" {
    type = string
}


variable "pve_node" {
    type = string
}
variable "ciuser" {
    default = "locthp"
}
variable "template_name" {
    type = string
    default = "cloudinit-ubuntu-22.04-template"
}

variable "proxmox_params" {
    type = object({
        pm_api_url = string
        pm_user = string
        pm_password = string
        pm_debug = bool
        pm_tls_insecure = bool
    })
}

variable "proxmox_vm_qemu_cpu" {
    type = object({
      cores = optional(number, 1)
      sockets = optional(number, 1) 
      type = optional(string, "x86-64-v2-AES")
    })
    default = {
        cores = 1
        sockets = 1
        type = "x86-64-v2-AES"
    }
}
variable "proxmox_vm_qemu_memory"{
    type = object({
      amount = number
    })
    default = {
        amount = 2048
    }
}
variable "proxmox_vm_qemu_networking"{
    type = object({
      ipconfig0 = string
      gw = optional(string, "172.16.1.1")
      subnet = optional(number, 24)
      nameservers = optional(string, "172.16.1.5 172.16.1.6")
      searchdomain = optional(string, ".")
    })
}
variable "proxmox_vm_qemu_disk" {
  type = map(object({
    disks = optional(object({
      scsi = optional(object({
        # disk0 (system disk)
        scsi0 = optional(object({
          disk = optional(object({
            backup     = optional(bool, true)
            cache      = optional(string, "")
            emulatessd = optional(bool, false)
            format     = optional(string, "raw")
            iothread   = optional(bool, true)
            replicate  = optional(bool, true)
            size       = optional(string, "25G")
            storage    = optional(string, "local-lvm")
          }), {})
        }), {})
      }), {})
    }), {})
  }))
  default = {
    "server1" = {
      disks = {
        scsi = {
          
        }
      }
    }
  }
}
