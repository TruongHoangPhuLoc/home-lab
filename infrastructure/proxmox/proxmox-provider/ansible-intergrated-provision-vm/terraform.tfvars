proxmox_params = {
  "pm_api_url": "https://172.16.1.253:8006/api2/json", 
  "pm_user": "root@pam", "pm_password": "password", 
  "pm_debug": true, 
  "pm_tls_insecure": true 
}
target_node = "geekom-dev"
instance_configruations = {
  k8s-master-01 = {
    cpu = {
      cores = 2
    }
    memory = {
      amount = 4096
    }
    networking = {
      ip = "172.16.1.110"
    }
  }
  # k8s-worker-01 = {
  #   cpu = {
  #       cores = 2
  #   }
  #   memory = {
  #       amount = 4096
  #   }
  #   networking = {
  #     ip = "172.16.1.111"
  #   }
  # }
  # k8s-worker-02 = {
  #   cpu = {
  #       cores = 2
  #   }
  #   memory = {
  #       amount = 4096
  #   }
  #   networking = {
  #       ip = "172.16.1.112"
  #   }
  # }
}
