# platform/CLAUDE.md

Rules for everything under `platform/`. Read this *in addition to* the root [`CLAUDE.md`](../CLAUDE.md) when adding or modifying components in this subtree. End-user workloads live in [`apps/`](../apps/) and follow [`apps/CLAUDE.md`](../apps/CLAUDE.md) — the patterns described here apply there too.

## Invariant

**Everything under `platform/` is ArgoCD-managed by default.** Components follow the Kustomize-wraps-Helm + SOPS/KSOPS pattern described below. Deliberate exceptions (helm-managed only) are listed at the bottom of this file.

## Subdirectory layout

```
platform/
├── argocd/              # ArgoCD itself (self-bootstrap docs live in argocd/BOOTSTRAP.md)
├── cert-manager/
├── cluster-api/
├── ingress/
│   └── traefik/
├── networking/
│   ├── cilium/          # EXCEPTION — helm-managed (CNI chicken-and-egg)
│   └── external-dns/
├── observability/
│   ├── agents/          # promtail, filebeat, node-exporter
│   ├── logging/         # loki
│   ├── monitoring/      # kube-prometheus-stack
│   └── tracing/         # tempo
└── storage/
    └── rook-ceph/       # EXCEPTION — deferred (see `project_rook_ceph_argocd_plan` memory)
```

## The canonical component directory

Every ArgoCD-managed component is a single directory with this exact shape:

```
platform/<category>/<component>/
├── application.yaml      # ArgoCD Application CRD
├── kustomization.yaml    # kustomize with helmCharts + (optional) generators
├── values.yaml           # helm overrides, plaintext, non-sensitive only
├── ksops.yaml            # OPTIONAL — only when secret.enc.yaml exists
└── secret.enc.yaml       # OPTIONAL — SOPS-encrypted Secret(s)
```

**Reference implementation:** [`platform/observability/agents/promtail/`](./observability/agents/promtail/).

### Canonical templates

Copy these when adding a new component and fill in the `<placeholders>`.

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

**`ksops.yaml`** (only when a component has encrypted secrets)
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
    # OMIT this finalizer for stateful components (rook-ceph, databases)
    # where Application deletion must NOT cascade-delete the underlying
    # resources. Include it for everything else.
    - resources-finalizer.argocd.argoproj.io
spec:
  project: default
  source:
    repoURL: https://github.com/TruongHoangPhuLoc/home-lab.git
    targetRevision: main
    path: platform/<category>/<component>
    plugin:
      name: ksops
  destination:
    server: https://kubernetes.default.svc
    namespace: <target-namespace>
  syncPolicy:
    automated:
      prune: false         # during adoption; flip to true in a SEPARATE commit after stable
      selfHeal: false      # during adoption; flip to true in a SEPARATE commit after stable
    syncOptions:
      - ServerSideApply=true
      - CreateNamespace=false      # set true only if creating a new namespace
      - RespectIgnoreDifferences=true
  # ignoreDifferences:
  #   - add as needed for fields an operator mutates at runtime (see patterns below)
```

## Non-negotiable rules

1. **No vendored helm charts.** Pull from upstream via `helmCharts.repo`. Don't commit `Chart.yaml`, `templates/`, `charts/`, or `values.schema.json`. *(Also a global rule — stated again here because it's most relevant here.)*
2. **Pin chart version explicitly.** `version: <x.y.z>`. Never `latest`, never absent.
3. **Secrets go in `secret.enc.yaml`.** SOPS-encrypted `kind: Secret`. Helm values reference the Secret by name (`existingSecret`, `secretName`, `extraEnvVarsSecret`, etc.). Never inline a secret value in `values.yaml`.
4. **`ServerSideApply=true` on every Application.** Required for clean co-management when adopting an existing helm release.
5. **Start conservative on sync policy.** `prune: false + selfHeal: false` at first. Promote to `true` only in a separate commit after 2–3 clean sync cycles with no unexpected drift.
6. **Application `metadata.name` matches helm `releaseName`.** Plain `<component>` (e.g., `promtail`, not `promtail-logging`).
7. **`kustomization.yaml` in every component directory — no exceptions.** Even for components with no secrets and no customization, uniformity beats minor savings.
8. **Applications live in git, not the ArgoCD UI.** Every Application definition is a committed `application.yaml`.

## Adoption workflow — migrate an existing helm release to ArgoCD

1. **Extract current state from the live release:**
   ```bash
   helm --kube-context home-cluster get values <release> -n <ns> -o yaml \
     > /tmp/<release>-values.yaml
   ```
2. **Split sensitive vs. non-sensitive values:**
   - Non-sensitive overrides → `values.yaml` (commit as plaintext)
   - API tokens, passwords, TSIG keys → wrap in `kind: Secret`, save as `secret.enc.yaml`, then `sops -e -i secret.enc.yaml`
   - Update `values.yaml` so the chart references the Secret by name (chart-specific pattern: `existingSecret`, `extraEnvVarsSecret`, etc.)
3. **Handle initialization dependencies** (if any):
   - If the workload requires external resources (certificates, external secrets), include both ongoing automation (CronJob) and bootstrap automation (one-time Job) in the same Application
   - Use ArgoCD sync waves: bootstrap resources get `argocd.argoproj.io/sync-wave: "0"`, dependents get `"1"` or higher
   - See `kube-prometheus-stack` migration in [`LESSONS.md`](./argocd/LESSONS.md) for the full bootstrap pattern
4. **Write the 3–5 files** per the canonical templates above.
5. **Commit + push** to the branch ArgoCD follows (`main`).
6. **Clean up any old broken Application** (common after the 2026-04-21 restructure — stale source paths):
   ```bash
   # If metadata.finalizers is set, remove it first to avoid cascade-deleting resources:
   kubectl --context home-cluster patch application <name> -n argocd --type json \
     -p '[{"op":"remove","path":"/metadata/finalizers"}]'
   kubectl --context home-cluster delete application <name> -n argocd
   ```
6. **Apply the new Application:**
   ```bash
   kubectl --context home-cluster apply -f platform/<category>/<component>/application.yaml
   ```
7. **Verify:**
   ```bash
   kubectl --context home-cluster get application <name> -n argocd \
     -o custom-columns='SYNC:.status.sync.status,HEALTH:.status.health.status'
   kubectl --context home-cluster get pod -n <target-namespace>   # existing pods AGE unchanged
   ```
8. **After 2–3 clean sync cycles:** flip `prune: true + selfHeal: true` in a separate commit.
9. **Stop running `helm upgrade` on this release from this point on.** ArgoCD drives changes now — mutate `values.yaml`, sync.

## Exceptions: helm-managed components under platform/

These components live under `platform/` but deliberately stay **helm-managed** (not ArgoCD-managed). Detect the difference by directory contents:

| Pattern | Contents | Lifecycle |
|---|---|---|
| ArgoCD-managed (default) | `application.yaml` + `kustomization.yaml` + `values.yaml` + optional secrets | ArgoCD sync on git change |
| Helm-managed (exception) | `values.yaml` + optional secrets only. **No** `application.yaml` or `kustomization.yaml`. | `helm upgrade <release> … -f values.yaml` |

### `platform/networking/cilium/`

**Why:** Cilium provides the cluster's CNI. ArgoCD itself depends on pod networking to run. If an ArgoCD sync misfires against cilium, the cluster loses networking → ArgoCD pods can't talk → you can't use ArgoCD to recover. You'd be forced into out-of-band recovery with `helm` + `kubectl`. Not worth the risk for the modest GitOps benefit.

**How it's run:**
```bash
helm --kube-context home-cluster upgrade cilium cilium/cilium \
  -n kube-system \
  -f platform/networking/cilium/values.yaml
```

**Never:** don't create an `application.yaml` here. If you think cilium should become ArgoCD-managed, this is a big architectural decision — raise it as its own discussion, don't slip it in.

### `platform/storage/rook-ceph/`

**Why (current state):** Deferred. Storage outage cascades across every PV-using workload (prometheus, loki, minio, TLS certs, …). When we do migrate, it requires very conservative ArgoCD settings (never auto-prune, never auto-self-heal, `ignoreDifferences` for operator-managed fields, `Delete=false` sync option on the `CephCluster`). See the `project_rook_ceph_argocd_plan` memory for the complete plan when it's time.

**How it's run currently:**
```bash
helm --kube-context home-cluster upgrade rook-ceph rook-release/rook-ceph \
  -n rook-ceph \
  -f platform/storage/rook-ceph/operator/values.yaml
helm --kube-context home-cluster upgrade rook-ceph-cluster rook-release/rook-ceph-cluster \
  -n rook-ceph \
  -f platform/storage/rook-ceph/cluster/values.yaml
```

**Order of operations when we migrate:** stateless apps first (external-dns, loki, promtail, homepage [done — see [`apps/homepage/`](../apps/homepage/)], cert-manager, traefik, kube-prometheus-stack) — THEN rook-ceph, by user direction.

## Common `ignoreDifferences` patterns

Add to this list as you encounter new ones.

| Resource | Field | Why |
|---|---|---|
| `apps/DaemonSet` | `/spec/templateGeneration` | Helm increments this on rolling updates; drifts independently of git state. Seen on promtail. |

## Common pitfalls during adoption

### Live resource has duplicate-keyed list items → `ComparisonError` blocks sync

ArgoCD with `ServerSideApply=true` uses structured merge diff, which requires every list-of-maps on the live resource to have unique values on its merge key. Two containers with the same name, two env vars with the same name, etc., will fail diff calculation with:

```
Failed to compare desired state to live state: failed to calculate diff:
  error calculating structured merge diff: error building typed value from
  live resource: .spec.template.spec.containers[name="X"].env:
    duplicate entries for key [name="HOSTNAME"]
```

This error happens *before* `ignoreDifferences` is applied — you can't suppress it from the Application spec. Fix the live resource itself:

```bash
kubectl --context <ctx> patch <kind>/<name> -n <ns> --type json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/env",
         "value":[ <correct env list> ]}]'
```

ArgoCD re-diffs on the next cycle (~3 min) or on manual Refresh.

**Seen on:** promtail DaemonSet, 2026-04-24. A historical chart bug had placed two identical `HOSTNAME` env entries on the container; kubelet accepted the duplicate but SSA didn't.

## When in doubt

1. Open [`platform/observability/agents/promtail/`](./observability/agents/promtail/) as the reference implementation.
2. Read [`platform/argocd/ARCHITECTURE.md`](./argocd/ARCHITECTURE.md) for the end-to-end render pipeline (Application → repo-server → KSOPS sidecar → kustomize → cluster) with diagrams. Useful when something works in `kubectl apply -f` but fails in ArgoCD.
3. Skim [`platform/argocd/LESSONS.md`](./argocd/LESSONS.md) for prior incidents with the same shape as what you're seeing.
4. Search the memory directory for component-specific notes (`project_*.md`).
5. Prefer asking before taking a destructive cluster action. Adoption should be idempotent — if it feels risky, it usually is.
