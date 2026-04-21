output "elasticstack-server-provision" {
  #value = tomap({for key, value in module.mail-server-provision.output_map: key => value})
  value = module.elasticstack-server-provision.output_map
}