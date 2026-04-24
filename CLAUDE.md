# CLAUDE.md

Instructions for AI agents (and humans) adding or modifying ArgoCD-managed components in this repo.

## Project in one paragraph

Personal homelab. Proxmox-hosted VMs + self-provisioned Kubernetes cluster + supporting platform/observability/automation stack. ArgoCD v3.3.8 runs in-cluster and manages most workloads via a **Kustomize-wraps-Helm + SOPS/KSOPS** pattern. Secrets are SOPS-encrypted (age) and committed as `*.enc.yaml` files; a KSOPS sidecar in argocd-repo-server decrypts at render time. Deeper context in [`README.md`](./README.md) and [`platform/argocd/BOOTSTRAP.md`](./platform/argocd/BOOTSTRAP.md).

## Top-level layout

| Directory | Purpose |
|---|---|
| `apps/` | End-user workloads (chat-app, homepage, …) |
| `platform/` | Cluster plumbing (argocd, cert-manager, ingress, storage, networking, cluster-api) |
| `observability/` | Monitoring, logging, tracing, agents — intentionally separate from `platform/` |
| `infrastructure/` | Bare-metal / VM layer (proxmox, networking, services, kubernetes install) |
| `automation/` | Config management + CI/CD (ansible, ci-cd) |

## The canonical Application directory

Every ArgoCD-managed component is one directory following this exact shape:

```
<tier>/<category>/<component>/
├── application.yaml      # ArgoCD Application CRD
├── kustomization.yaml    # kustomize with helmCharts + (optional) generators
├── values.yaml           # helm overrides, plaintext, non-sensitive only
├── ksops.yaml            # OPTIONAL — only if the component has secrets
└── secret.enc.yaml       # OPTIONAL — SOPS-encrypted Secret(s)
```

Example: [`observability/agents/promtail/`](./observability/agents/promtail/).

### Canonical templates (copy these when adding a new component)

**`values.yaml`**
```yaml
# <component> helm overrides.
# Pinned chart: <chart-name> <version> from <upstream-repo>.
# Add overrides only when something needs to change from chart defaults.
```

**`kustomization.yaml`**
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

helmCharts:
  - name: <chart-name>
    repo: https://<upstream-chart-repo>
    version: <pinned-version>
    releaseName: <release-name>
    namespace: <target-namespace>
    valuesFile: values.yaml

# Add this block ONLY if the component has secrets:
# generators:
#   - ./ksops.yaml
```

**`ksops.yaml`** (only when secrets exist)
```yaml
apiVersion: viaduct.ai/v1
kind: ksops
metadata:
  name: <component>-secrets
  annotations:
    config.kubernetes.io/function: |
      exec:
        path: ksops
files:
  - ./secret.enc.yaml
```

**`application.yaml`**
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: <component>
  namespace: argocd
  finalizers:
    # OMIT for stateful components (rook-ceph, databases with data);
    # include for everything else so deletion prunes cleanly.
    - resources-finalizer.argocd.argoproj.io
spec:
  project: default
  source:
    repoURL: https://github.com/TruongHoangPhuLoc/home-lab.git
    targetRevision: main
    path: <tier>/<category>/<component>
    plugin:
      name: ksops
  destination:
    server: https://kubernetes.default.svc
    namespace: <target-namespace>
  syncPolicy:
    automated:
      prune: false         # flip to true only after stable for several sync cycles
      selfHeal: false      # flip to true only after adoption verified
    syncOptions:
      - ServerSideApply=true
      - CreateNamespace=false        # set true only if creating a new namespace
      - RespectIgnoreDifferences=true
  # ignoreDifferences:
  #   - add as needed for fields an operator mutates at runtime
```

## Non-negotiable rules

1. **No vendored helm charts.** Pull from upstream via `helmCharts.repo`. Never commit `Chart.yaml`, `templates/`, `charts/`, or `values.schema.json`.
2. **Pin chart versions explicitly.** `version: <x.y.z>`. Never `latest`.
3. **Secrets go in `secret.enc.yaml`.** SOPS-encrypted `kind: Secret`. Helm values reference them by name (`existingSecret`, `secretName`, etc.), never inline.
4. **`ServerSideApply=true` on every Application.** Required for clean co-management with existing helm releases.
5. **Start conservative on sync policy.** `prune: false + selfHeal: false` at first. Promote to `true` only in a separate commit after 2–3 clean sync cycles.
6. **Application name matches helm `releaseName`.** Plain `<component>` (e.g., `promtail`), not `<component>-<namespace>`.
7. **Applications live in git.** Do not create Applications via the ArgoCD UI — every Application is defined by a committed `application.yaml`.
8. **Kustomization.yaml in every component directory.** Even for components with no secrets. Uniformity > minor savings.

## What NOT to bring under ArgoCD

| Component | Why | Where it lives |
|---|---|---|
| `cilium` (CNI) | Chicken-and-egg — ArgoCD needs CNI to run. Bad sync = cluster dead. | Helm-managed; values in git under `platform/networking/cilium/` (planned). |
| `rook-ceph` / `rook-ceph-cluster` | Storage outage cascades everywhere. Deferred until all stateless apps landed. | Plan in the `project_rook_ceph_argocd_plan` memory; co-management approach with very conservative policy when we get there. |
| `ingress-nginx` | Being phased out in favour of cilium + traefik. | Leave alone; don't invest work in it. |

## Secrets workflow (quick ref — full guide in BOOTSTRAP.md)

```bash
export SOPS_AGE_KEY_FILE="$HOME/.config/sops/age/keys.txt"  # put in ~/.bashrc

sops new.enc.yaml           # create + edit encrypted file ($EDITOR opens plaintext)
sops existing.enc.yaml      # edit in place (decrypt → edit → re-encrypt on save)
sops -d file.enc.yaml       # print decrypted to stdout
sops updatekeys file.enc.yaml  # re-encrypt after .sops.yaml changes
```

- `.sops.yaml` at repo root defines what gets encrypted (path_regex `*.enc.yaml`, encrypted_regex `^(data|stringData)$`)
- Public key: `age1f2ga2qhdv6hpfhlelk7t633yzh78u4jdkwxkxrcpml5a7tzyd9ps99zmkj` (safe to commit)
- Private key lives on the operator workstation + password-manager backup

## Adoption workflow — migrate an existing helm release to ArgoCD

1. **Extract current state**
   ```bash
   helm get values <release> -n <ns> -o yaml > /tmp/current-values.yaml
   ```
2. **Split sensitive vs. non-sensitive fields**
   - Non-sensitive helm knobs → `values.yaml`
   - API tokens, passwords, TSIG keys, etc. → wrap in `kind: Secret`, save as `secret.enc.yaml`, then `sops -e -i secret.enc.yaml`
   - Update `values.yaml` so the chart references the Secret by name (chart-specific: `existingSecret`, `extraEnvVarsSecret`, etc.)
3. **Write the 3–5 files** per the canonical templates above.
4. **Commit + push**.
5. **Clean up any old broken Application** (common after the 2026-04-21 restructure — old path references):
   ```bash
   # If it has finalizers, remove them first to avoid cascade-deleting resources:
   kubectl patch application <name> -n argocd --type json \
     -p '[{"op":"remove","path":"/metadata/finalizers"}]'
   kubectl delete application <name> -n argocd
   ```
6. **Apply the new Application**
   ```bash
   kubectl apply -f <tier>/<category>/<component>/application.yaml
   ```
7. **Verify**
   ```bash
   kubectl get application <name> -n argocd \
     -o custom-columns='SYNC:.status.sync.status,HEALTH:.status.health.status'
   kubectl get pod -n <target-namespace>    # AGE should be unchanged for existing pods
   ```
8. **After 2–3 clean sync cycles**: flip `prune: true + selfHeal: true` in a separate commit.
9. **Never run `helm upgrade` on this release again** — ArgoCD drives changes now. Use `git` + ArgoCD sync.

## Where to find deeper info

| Topic | File |
|---|---|
| SOPS setup, key rotation, recovery, safety checks | [`platform/argocd/BOOTSTRAP.md`](./platform/argocd/BOOTSTRAP.md) |
| Repo overview, top-level layout, achievements | [`README.md`](./README.md) |
| SOPS encryption rules | [`.sops.yaml`](./.sops.yaml) |
| git-ignore rules (note the `!*.enc.yaml` negation) | [`.gitignore`](./.gitignore) |
