output "mail-server-output" {
  #value = tomap({for key, value in module.mail-server-provision.output_map: key => value})
  value = module.mail-server-provision.output_map
}