output "instances_ip" {
  value = [for instance in var.instance_configruations : instance.networking.ip]
}