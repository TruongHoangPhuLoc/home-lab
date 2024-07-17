proxmox_params = {"pm_api_url": "https://172.16.1.253:8006/api2/json", "pm_user": "root@pam", "pm_password": "Phuloc@99.", "pm_debug": true, "pm_tls_insecure": true }
proxmox_vm_qemu_disk = {
  disks = {
    scsi = {
      # disk1 (optional)
      scsi1 = {
        disk = {
          size       = "20G"
          storage    = "local-lvm" 
          iothread   = true
        }
      }
      # disk2 (optional)
      scsi2 = {
        disk = {
          size       = "20G"
          storage    = "local-lvm" 
          iothread   = true
        }
      }
    }
  }
}