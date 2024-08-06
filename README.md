# home-lab
Achievements:
+ Created my own k8s clusters on-premise by both manuall and automatically-provisioned methods
+ Gained experience with Ansible, Terraform
+ Fine-tuned Proxmox cluster regarding to my needs
+ Experimented open-source solutions related to System/Linux/LB/Monitoring/SSL Certs
+ Achieved more comprehensive knowledge about Linux to support my job by reproducing the issue from work (SSL Certs, Kernel panic, Disk Usage Monitoring,...) and tried my best to resolve as much as possible
+ Had more experience in scripting
+ Built a control node to automatically update/patch target servers weekly, updating my discord channel with the result of update, even if there's a failure when update.
+ Setup, designed my own network architect(basic) using available solutions:
    + Opnsense for access control my private network
    + Cloud Flare tunnels for accessing my private from Internet
    + Created private DNS zone with separate DNS authoritative servers (master-slave strategy) and forwarder servers (synchronized PiHole instances)
    + Implemented a mail server for alerting
+ Built automated-provision k8s cluster being like cloud-provisoned ones with following abilities:
    + Dynamically provision PVC/PV based on storage class (Longhorn)
    + Dynamically provision External IPs for LoadBalancer services (Metallb, Bird, Opnsense)
    + Handle traffic at L7 layer (Ingress controller)
    + SSL certs auto-renewal for the soon-expiring ones(Cert Manager)
    + Be able to access services from the Internet(Cloud Flare tunnel technologies)
    + Experimented and experienced in fine-tunning ECMP decision at BGP router, to avoid disrupted connection when neighbors added/deleted
+ Monitoring:
    + Set a Monitoring Stack up to observe all my own servers
    + Forged a CD flow to automate the deployment when having new changes
    + Integrated Github with Jenkins hosted on private network

