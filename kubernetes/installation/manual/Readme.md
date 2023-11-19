# An explanation of how my cluster's infrastucture is

![Alt text](diagram.png)

My cluster consists of 4 nodes with 1 control-plane and 3 workers

Due to bare metal environment, there're some aspects I want to my cluster such as:
+ External Storage (pods are ephemeral, I need data somehow could be persisted across recreation)
+ External Load-Balancer IP range (autmatic provisioning an IP address based on defined range for newly created service)