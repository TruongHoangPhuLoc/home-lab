output "haproxy-output" {
  #value = tomap({for key, value in module.mail-server-provision.output_map: key => value})
  value = module.haproxy-server-provision.output_map
}