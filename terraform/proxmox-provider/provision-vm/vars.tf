variable "ssh_keys" {
  type = string
  default = <<-EOT
  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHgKmDqIR8VZ+sMoCxjt8HTlerwO29A7MS4lQMNehsr3 root@tasks-automation-server
  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIA61Dt7OxM8Jpoy/I0/FmCLjaqjNApU+UO+vRpyavBoj truonghoangphuloc@phus-MacBook-Pro.local
  EOT
}

# variable "prefix_name" {
#     type = string
# }
# variable "vm_amount" {
#   type = number
# }

variable "target_node" {
    type = string
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
variable "instance_configruations" {
  type = map(object({
    vmid = string
    cpu = optional(object({
      cores = optional(string, "1")
      sockets = optional(string, "1")
      type = optional(string, "x86-64-v2-AES")
    }), {})
    memory = optional(object({
      amount = optional(string, "2048")
    }), {})
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
        # disk1 (optional)
        scsi1 = optional(object({
          disk = optional(object({
            backup     = optional(bool)
            cache      = optional(string)
            emulatessd = optional(bool)
            format     = optional(string)
            iothread   = optional(bool)
            replicate  = optional(bool)
            size       = optional(string)
            storage    = optional(string, "local-lvm")
          }), {})
        }), {})
        # disk2 (optional)
        scsi2 = optional(object({
          disk = optional(object({
            backup     = optional(bool)
            cache      = optional(string)
            emulatessd = optional(bool)
            format     = optional(string)
            iothread   = optional(bool)
            replicate  = optional(bool)
            size       = optional(string)
            storage    = optional(string, "local-lvm")
          }), {})
        }), {})
      }), {})
    }), {})
    networking = object({
      ip = string
      subnet = optional(string, "24")
      nameservers = optional(string, "172.16.1.5 172.16.1.6")
      gateway = optional(string, "172.16.1.1")
      searchdomain = optional(string, ".")
    })
  }))
}
variable "misc" {
  type = object({
    ciuser = optional(string, "locthp")
    template = optional(string, "cloudinit-ubuntu-22.04-template" )
  })
  default = {}
}