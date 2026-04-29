# Traefik Ingress Controller

ArgoCD-managed deployment of Traefik v3 ingress controller using the Kustomize-wraps-Helm + KSOPS pattern.

## Overview

Traefik serves as the primary ingress controller for the home-cluster, providing:
- **HTTP/HTTPS Ingress**: Routes external traffic to Kubernetes services
- **Automatic TLS**: Integration with cert-manager for Let's Encrypt certificates
- **Gateway API Support**: Modern Kubernetes Gateway API alongside traditional Ingress
- **Dashboard**: Web UI for monitoring and configuration
- **Metrics**: Prometheus metrics integration for observability

## Architecture

### Network Flow
```
Internet → Edge Router → Cilium BGP → Traefik LoadBalancer → Backend Services
```

**Current Network Architecture:**
- **Direct BGP Routing**: Cilium BGP advertises LoadBalancer service IP ranges
- **No Proxy Layer**: Traffic flows directly from edge router to Traefik
- **Client IP Preservation**: Real client IPs naturally preserved (no ProxyProtocol needed)
- **LoadBalancer Service**: `172.16.180.1` (advertised via BGP)

### Deployment Pattern
- **Base**: Helm chart `traefik/traefik` v39.0.7 (app version v3.6.12)
- **Customization**: Kustomize overlays with SOPS-encrypted secrets
- **Management**: ArgoCD Application with manual sync (migration phase)
- **Dependencies**: cert-manager for TLS certificates

### File Structure
```
platform/networking/ingress/traefik/
├── application.yaml      # ArgoCD Application definition
├── kustomization.yaml    # Helm chart + KSOPS configuration  
├── values.yaml          # Non-sensitive Helm values + network config
├── secret.enc.yaml      # SOPS-encrypted dashboard credentials
├── ksops.yaml           # KSOPS generator config
└── README.md           # This file
```

## Configuration

### Core Features
- **Default IngressClass**: `traefik` (cluster default)
- **Gateway API**: Enabled alongside traditional Ingress
- **HTTPS Redirect**: All HTTP traffic automatically redirects to HTTPS
- **Dashboard**: Available at `traefik.prod-cluster.internal.locthp.com`
- **Access Logs**: Enabled for monitoring and debugging
- **Prometheus Metrics**: Exposed for monitoring stack integration

### ProxyProtocol Configuration
**Current Status**: DISABLED (commented out)

ProxyProtocol is currently disabled because traffic flows directly from the edge router via Cilium BGP to Traefik without any intermediate proxy. Client IPs are naturally preserved.

**When to Enable**: If you add a proxy/load balancer in front of Traefik in the future:

1. Uncomment the `proxyProtocol` sections in `values.yaml`
2. Adjust `trustedIPs` to match your proxy's IP ranges
3. Apply the configuration

Example:
```yaml
ports:
  web:
    proxyProtocol:
      trustedIPs:
      - 10.0.0.0/8       # Your proxy network
      - 172.16.0.0/12    # Container networks
```

### TLS Configuration
- **Wildcard Certificate**: `*.internal.locthp.com`, `*.prod-cluster.internal.locthp.com`
- **Dashboard Certificate**: `traefik.prod-cluster.internal.locthp.com`
- **Issuer**: Let's Encrypt Production (`letsencrypt-prod` ClusterIssuer)
- **Auto-renewal**: Managed by cert-manager

## Migration History

**Migrated**: 2026-04-29 from Helm-managed to ArgoCD-managed  
**Previous State**: Direct `helm install traefik/traefik`  
**Migration Commits**: `f9e4d53`, `bd822f5`

### Key Changes During Migration
1. **Secret Management**: Dashboard credentials moved to SOPS-encrypted secret
2. **ProxyProtocol**: Disabled and documented for current BGP architecture
3. **Configuration Split**: Sensitive vs non-sensitive values properly separated
4. **ArgoCD Integration**: Full GitOps workflow with KSOPS for secret decryption

## Secret Management

### Encrypted Secrets (`secret.enc.yaml`)
SOPS-encrypted with age key, contains:
- `username`: Dashboard admin username
- `password`: Dashboard admin password

### Authentication
Dashboard uses BasicAuth middleware referencing the SOPS-managed secret.

## Network Configuration

### Service Configuration
- **Type**: LoadBalancer
- **External IP**: `172.16.180.1` (assigned by Cilium BGP)
- **Ports**: 
  - HTTP: `80` → `8000` (redirects to HTTPS)
  - HTTPS: `443` → `8443`

### Ingress Classes
- **traefik**: Default IngressClass for the cluster
- **Gateway API**: Enabled for modern ingress configurations

### DNS Integration
- **external-dns**: Automatically creates DNS records
- **Target IP**: Points to `172.16.180.1` (LoadBalancer external IP)

## Operations

### Deployment
```bash
# Apply the ArgoCD Application
kubectl apply -f application.yaml

# Monitor sync status
kubectl get application -n argocd traefik -w

# Manual sync (during migration phase)
kubectl patch application traefik -n argocd --type merge -p '{"operation":{"sync":{"syncStrategy":{"hook":{},"apply":{"force":false}}}}}'
```

### Dashboard Access
1. **URL**: https://traefik.prod-cluster.internal.locthp.com
2. **Authentication**: BasicAuth (credentials in SOPS secret)
3. **Features**: Real-time metrics, routing configuration, middleware status

### Monitoring
```bash
# Check Traefik pods
kubectl get pods -n traefik

# View access logs
kubectl logs -n traefik deployment/traefik -f

# Check LoadBalancer status
kubectl get svc -n traefik traefik

# Verify certificates
kubectl get certificates -n traefik
```

### Configuration Updates

#### Updating Helm Values
Edit `values.yaml` for non-sensitive changes:
```bash
# Edit configuration
vim platform/networking/ingress/traefik/values.yaml

# Commit changes
git add . && git commit -m "traefik: update configuration"

# ArgoCD will auto-sync (once migration complete)
```

#### Updating Secrets
Edit dashboard credentials:
```bash
# Edit encrypted secrets (requires SOPS key)
sops platform/networking/ingress/traefik/secret.enc.yaml

# Commit and sync
git add . && git commit -m "traefik: update dashboard credentials"
```

#### Enabling ProxyProtocol
If adding a proxy in front of Traefik:
```bash
# 1. Edit values.yaml - uncomment proxyProtocol sections
# 2. Update trustedIPs to match your proxy network
# 3. Commit and apply changes
```

### Troubleshooting

#### Application Sync Issues
```bash
# Check Application status
kubectl describe application traefik -n argocd

# Check repo-server logs for KSOPS issues
kubectl logs -n argocd -l app.kubernetes.io/name=argocd-repo-server -c ksops

# Force refresh
kubectl patch application traefik -n argocd --type merge -p '{"operation":{"sync":{"syncStrategy":{"hook":{},"apply":{"force":true}}}}}'
```

#### LoadBalancer Issues
```bash
# Check BGP advertisements (if using Cilium BGP)
kubectl get bgppolicy -A

# Verify LoadBalancer assignment
kubectl describe svc traefik -n traefik

# Check Cilium status
cilium status
```

#### Certificate Issues
```bash
# Check certificate status
kubectl describe certificate -n traefik

# Check cert-manager logs
kubectl logs -n cert-manager deployment/cert-manager

# Manual certificate request
kubectl delete certificate traefik-default-cert -n traefik
# (cert-manager will recreate automatically)
```

#### Dashboard Access Issues
```bash
# Verify secret exists and is decrypted
kubectl get secret dashboard-auth-secret -n traefik -o yaml

# Check middleware configuration
kubectl get middleware -n traefik dashboard-auth -o yaml

# Test authentication
curl -u admin:password https://traefik.prod-cluster.internal.locthp.com/dashboard/
```

## Integration Points

### ArgoCD
- **Application Path**: `platform/networking/ingress/traefik`
- **Sync Policy**: Manual (during migration), will be automated post-migration
- **Plugin**: KSOPS for secret decryption
- **Namespace**: `traefik`

### Monitoring Stack
- **Metrics**: Prometheus scraping enabled
- **ServiceMonitor**: Auto-discovered by kube-prometheus-stack
- **Grafana**: Traefik dashboard available in monitoring stack

### Certificate Management
- **cert-manager**: Automatic certificate provisioning and renewal
- **ClusterIssuer**: `letsencrypt-prod`
- **DNS Challenge**: Configured for wildcard certificates

### External DNS
- **Provider**: Configured to create DNS records automatically
- **Target**: `172.16.180.1` (LoadBalancer external IP)
- **Annotation**: `external-dns.alpha.kubernetes.io/target`

## Migration from Helm to ArgoCD

If migrating another Traefik instance:

1. **Extract Configuration**: `helm get values traefik -n traefik -o yaml`
2. **Split Values**: Sensitive → `secret.enc.yaml`, non-sensitive → `values.yaml`
3. **Encrypt Secrets**: `sops -e -i secret.enc.yaml`
4. **Create ArgoCD Files**: `kustomization.yaml`, `ksops.yaml`, `application.yaml`
5. **Test Sync**: Apply Application with manual sync
6. **Verify Functionality**: Check dashboard, ingress routing, certificates
7. **Remove Helm**: `helm uninstall traefik -n traefik --keep-history`
8. **Enable Auto-sync**: Update Application for automated GitOps

## Component Documentation

- **Design Document**: [`DESIGN.md`](./DESIGN.md) - Complete design decisions and architectural reasoning for this component
- **ArgoCD Architecture**: [`platform/argocd/ARCHITECTURE.md`](../../argocd/ARCHITECTURE.md) - How ArgoCD + KSOPS + Kustomize works
- **Platform Conventions**: [`platform/CLAUDE.md`](../../CLAUDE.md) - General patterns and adoption workflow
- **Network Architecture**: Check `infrastructure/` for BGP and Cilium configuration details

## References

- **Upstream Helm Chart**: [traefik/traefik](https://github.com/traefik/traefik-helm-chart)
- **Traefik Documentation**: [doc.traefik.io](https://doc.traefik.io/traefik/)
- **Gateway API**: [gateway-api.sigs.k8s.io](https://gateway-api.sigs.k8s.io/)
- **SOPS Documentation**: [Mozilla/sops](https://github.com/mozilla/sops)
- **KSOPS Plugin**: [viaduct-ai/kustomize-sops](https://github.com/viaduct-ai/kustomize-sops)