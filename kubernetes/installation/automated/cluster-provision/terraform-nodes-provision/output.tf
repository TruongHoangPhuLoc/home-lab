output "ips" {
  value = concat(module.k8s-master-nodes-provision.instances_ip, module.k8s-worker-nodes-provision.instances_ip)
}