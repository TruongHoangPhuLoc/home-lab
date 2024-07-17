variable "ssh_keys" {
  default =  <<EOT
  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHgKmDqIR8VZ+sMoCxjt8HTlerwO29A7MS4lQMNehsr3 root@tasks-automation-server
  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIA61Dt7OxM8Jpoy/I0/FmCLjaqjNApU+UO+vRpyavBoj truonghoangphuloc@phus-MacBook-Pro.local
  EOT
}
variable "proxmox_host" {
    default = "dell-03"
}
variable "template_name" {
    default = "cloudinit-ubuntu-22.04-template"
}