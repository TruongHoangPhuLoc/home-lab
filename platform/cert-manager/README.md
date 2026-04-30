# cert-manager — operations

Day-to-day commands. The *why* lives in [`clusterissuer.yaml`](./clusterissuer.yaml) comments and the migration commit (`9cdccba`).

## What this directory provides

- **cert-manager** controllers (controller, webhook, cainjector) deployed via the upstream Jetstack chart pinned to v1.18.2.
- **`letsencrypt-prod` ClusterIssuer** — ACME issuer using Let's Encrypt with Cloudflare DNS-01.
- **`cloudflare-api-key-secret`** — SOPS-encrypted Cloudflare Global API Key, mounted into cert-manager for solving the DNS challenge.

DNS-01 (vs HTTP-01) is required because most homelab services live behind cilium/traefik on a private LB IP that isn't reachable from the public internet — Let's Encrypt can't HTTP-validate them, but it can read the `_acme-challenge` TXT record cert-manager places on Cloudflare.

## Lifecycle

ArgoCD-managed (no `helm upgrade` from this point on). Mutate via git:

```bash
# After editing values.yaml / clusterissuer.yaml / etc.:
git add platform/cert-manager
git commit -m "cert-manager: <change>"
git push
# ArgoCD syncs within ~3 minutes; force with the UI Refresh button if impatient.
```

The Application is at `argocd/cert-manager`, configured with `prune: true + selfHeal: true` after adoption verified clean.

## Verifying a fresh issue

```bash
# All Certificates across the cluster (status, age):
kubectl get certificates -A

# Inspect a specific Certificate (look for 'Ready: True'):
kubectl describe cert <name> -n <namespace>

# Inspect the underlying CertificateRequest / Order / Challenge during issuance:
kubectl get certificaterequest,order,challenge -A
```

A typical successful issuance flow:

```
Certificate           ┐
  └─ CertificateRequest ┐
       └─ Order           ┐
            └─ Challenge   ─→ Cloudflare TXT record placed
                                ↓
                          ACME validates DNS
                                ↓
                          Order completed → Certificate Ready=True → Secret populated
```

If a Certificate sits in `Issuing` for more than ~2 minutes, look at `kubectl describe challenge <name> -n <ns>` for the failure reason — usually a Cloudflare API error, a DNS propagation issue, or a misconfigured zone.

## Adding a TLS cert to a workload

Two patterns:

**Via Ingress annotation** (preferred — cert-manager generates the Certificate CR for you):

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    external-dns.alpha.kubernetes.io/hostname: my-app.prod-cluster.internal.locthp.com
spec:
  ingressClassName: cilium
  tls:
    - hosts: [my-app.prod-cluster.internal.locthp.com]
      secretName: my-app-tls
  rules:
    - host: my-app.prod-cluster.internal.locthp.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-app
                port: { number: 80 }
```

**Via explicit Certificate CR** (when you need a TLS Secret without an Ingress, e.g., a custom server, mTLS):

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: my-app-cert
  namespace: my-app
spec:
  secretName: my-app-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
    - my-app.prod-cluster.internal.locthp.com
```

## Rotating the Cloudflare API key

If the key needs to be rotated (Cloudflare admin → API Tokens → revoke + regenerate):

```bash
cd platform/cert-manager
sops secret.enc.yaml          # opens decrypted in $EDITOR; replace api-key value, save, exit
git add secret.enc.yaml
git commit -m "cert-manager: rotate Cloudflare API key"
git push
# ArgoCD reconciles → cert-manager pods pick up the new value via SSA on the Secret.
# Pods don't auto-restart on Secret change — bounce them if you need an immediate pickup:
kubectl rollout restart deploy/cert-manager -n cert-manager
```

Eventually we should swap from Global API Key (full account access) to a scoped API Token (`Zone:DNS:Edit` only). Easy follow-up:

1. Create a Cloudflare API Token with the `Zone:DNS:Edit` permission scoped to `locthp.com`.
2. In `secret.enc.yaml`, rename the key from `api-key` to `api-token`.
3. In `clusterissuer.yaml`, change `apiKeySecretRef` → `apiTokenSecretRef`.
4. Commit + push; ArgoCD reconciles the issuer and re-registers with the new auth.

## Troubleshooting

| Symptom | First check |
|---|---|
| Certificate stuck `Issuing` | `kubectl describe challenge <name>` for the Cloudflare/ACME error message |
| `cloudflare-api-key-secret not found` | OBC drift after a sync misfire — `kubectl get app cert-manager -n argocd -o yaml` for sync state, then re-sync |
| ClusterIssuer `Ready=False` | Cloudflare token expired/revoked; rotate per above |
| ACME rate-limited (5 certs/week) | Use Let's Encrypt staging during testing; switch the ClusterIssuer to `https://acme-staging-v02.api.letsencrypt.org/directory` |
| Pod won't start, webhook not ready | The chart's `startupapicheck` Job validates webhook reachability; check its pod logs in the `cert-manager` ns |
| TLS Secret older than the cert says it should be | cert-manager waits for `renewBefore` (default 360h before expiry) — bump renewal manually with `cmctl renew <cert> -n <ns>` |

## Related docs

- [`clusterissuer.yaml`](./clusterissuer.yaml) — the ACME issuer spec, comments explain the auth choice.
- [`platform/argocd/ARCHITECTURE.md`](../argocd/ARCHITECTURE.md) — how ArgoCD renders this directory.
- [Upstream cert-manager docs](https://cert-manager.io/docs/) — comprehensive reference.
