output "cluster_output" {
  value = { "${var.clustername}" : module.k8s-nodes-provision.output_map }
}