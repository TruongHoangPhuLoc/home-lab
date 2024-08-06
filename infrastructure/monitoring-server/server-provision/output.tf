output "monitoring-test-output" {
  #value = tomap({for key, value in module.mail-server-provision.output_map: key => value})
  value = module.monitoring-test-provision.output_map
}