# home-lab

A personal homelab for learning — mixes Proxmox-hosted VMs, a self-provisioned Kubernetes cluster, and the supporting network / observability / storage stack around them. This repo tracks the infrastructure-as-code, k8s manifests, and automation that runs it.

## Repository layout

```
apps/              Workloads (chat-app, homepage, ...)
platform/          Cluster platform services (helm overrides, future ArgoCD Applications)
observability/     Monitoring / logging / tracing stack
infrastructure/    Bare-metal / VM layer — Proxmox, networking (OPNsense/BIND/HAProxy),
                   docker-based services, kubernetes install scripts
automation/        Config management & CI/CD
  ├── ansible/     inventory, playbooks/, templates/, compose-files/
  └── ci-cd/       Jenkinsfile
```

Most cluster platform services (ArgoCD, cert-manager, cilium, ingress-nginx, traefik, kube-prometheus-stack, loki, promtail, rook-ceph) currently run as in-cluster Helm releases rather than being fully declared in git — see "Current direction" below.

## Current direction

- **ArgoCD migration** — bringing existing Helm releases under ArgoCD `Application` CRDs so the cluster state is fully in git rather than imperatively installed. `ingress-nginx` is excluded from this work — it is being phased out in favour of Cilium + Traefik.
- **Terraform state to Ceph S3** — moving Terraform `tfstate` files off local disk into the in-cluster Rook-Ceph RGW as a durable S3-compatible backend.

## Secret management

Secrets are encrypted with [SOPS](https://github.com/getsops/sops) using [age](https://github.com/FiloSottile/age) and committed to the repo as `*.enc.yaml` files. A KSOPS sidecar in `argocd-repo-server` decrypts them at render time. Configuration is in [`.sops.yaml`](./.sops.yaml) at the repo root.

Public key (safe to share):
```
age1f2ga2qhdv6hpfhlelk7t633yzh78u4jdkwxkxrcpml5a7tzyd9ps99zmkj
```

See [`platform/argocd/BOOTSTRAP.md`](./platform/argocd/BOOTSTRAP.md) for setup, daily workflow, and recovery procedures.

## Achievements

- Created on-premise Kubernetes clusters using both manual (kubeadm) and automated (Terraform + Ansible) methods.
- Gained experience with Ansible and Terraform for real provisioning flows.
- Fine-tuned a Proxmox cluster to fit homelab needs.
- Explored open-source solutions across systems / Linux / load-balancing / monitoring / SSL certs.
- Reproduced and resolved production-style Linux issues (SSL certs, kernel panic, disk-usage monitoring) as a learning loop for work.
- Built a control node that weekly patches target servers and reports the results (including failures) to a Discord channel.
- Designed a basic network architecture:
  - OPNsense for private-network access control.
  - Cloudflare Tunnels for exposing selected services to the internet.
  - Private DNS zone with master/slave authoritative servers plus synchronized PiHole forwarders.
  - Mail server for alerting.
- Built an automated-provision k8s cluster with cloud-like capabilities:
  - Dynamic PVC/PV provisioning via storage class (historically Longhorn, now Rook-Ceph).
  - Dynamic LoadBalancer External IPs (MetalLB + BIRD + OPNsense).
  - L7 traffic handling (ingress-nginx).
  - Auto-renewal of SSL certs (cert-manager).
  - Internet access via Cloudflare Tunnels.
  - Fine-tuned ECMP on the BGP router to avoid connection drops on neighbor changes.
- Monitoring:
  - Stood up a monitoring stack (kube-prometheus-stack, Loki, Promtail) across servers and cluster.
  - CD flow that deploys on new changes.
  - GitHub ↔ Jenkins integration with Jenkins hosted on the private network.
