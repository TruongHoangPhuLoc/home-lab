# output "instances_object" {
#   value = [for key, value in var.instance_configruations : {
#     instance_name =  key
#     instance_ip   =  value.networking.ip
#   }]
# }

output "output_map" {
  value = tomap({ for key, value in var.instances_configurations: key => value.networking.ip  })
}