output "monitoring-output" {
  #value = tomap({for key, value in module.mail-server-provision.output_map: key => value})
  value = module.monitoring-server-provision.output_map
}