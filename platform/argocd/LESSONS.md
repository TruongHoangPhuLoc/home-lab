# ArgoCD lessons learned

A running log of incidents involving ArgoCD, SOPS, KSOPS, and the kustomize render pipeline. Reverse-chronological. Each entry is short — two paragraphs and an explicit fix — but pinned to a date and a commit so future-me can reconstruct context.

If the same root cause shows up twice, **promote it** out of here into one of:
- [`ARCHITECTURE.md`](./ARCHITECTURE.md) — if it's about how the system fundamentally works.
- [`platform/CLAUDE.md`](../CLAUDE.md) → "Common pitfalls during adoption" — if it's a pattern to watch for during a migration.
- [`apps/CLAUDE.md`](../../apps/CLAUDE.md) → "Non-negotiable rules" — if it's a prevention rule.

## Entry template

```
## YYYY-MM-DD — short title

**What happened**: one paragraph, what the user-visible symptom was.

**Root cause**: one paragraph, the actual underlying cause (not the symptom).

**Fix**: bullets, what we changed.

**Why it took the time it took** (optional): one paragraph if there was a meaningful misdirection or learning.

**Prevention going forward**: bullets, links to docs/rules updated.

**References**: PR or commit links, related issues, helpful upstream issues.
```

---

## 2026-04-28 — kube-prometheus-stack migration: Helm-to-ArgoCD with bootstrap automation

**What happened**: Migrating `kube-prometheus-stack` from Helm-managed to ArgoCD-managed revealed a complex multi-layered challenge: (1) Kustomize-wraps-Helm + KSOPS pattern adoption, (2) IngressClass migration from `traefik` to `cilium`, (3) dynamic etcd certificate management integration, (4) Helm ownership handoff without resource conflicts, and (5) bootstrap automation for initialization dependencies. Each piece worked individually but the integration exposed race conditions and manual steps that violated GitOps principles.

**Root cause**: Multiple interconnected issues stemming from a sophisticated migration:

1. **Secret dependency chicken-and-egg**: Prometheus StatefulSet required `etcd-client-certs` secret for etcd metrics scraping, but the secret was managed by a CronJob that only ran daily at midnight. Fresh deployments failed with `MountVolume.SetUp failed for volume "secret-etcd-client-certs": secret "etcd-client-certs" not found` until manual intervention.

2. **Dual ownership conflict**: Helm release remained active while ArgoCD Application was deployed, creating resource ownership ambiguity. Both systems thought they owned the same Kubernetes objects, leading to potential drift and management conflicts.

3. **Bootstrap timing gap**: No mechanism existed to ensure the etcd certificates were available before Prometheus attempted to mount them, requiring manual `kubectl create job --from=cronjob/etcd-cert-syncer` on every fresh deployment.

**Fix** (commits `58bf457`, `b98794b`, `bb3503d`):

- **Kustomize-wraps-Helm structure**: Created `platform/observability/monitoring/kube-prometheus-stack/` following the standard pattern with `values.yaml` (non-sensitive overrides), `secret.enc.yaml` (SOPS-encrypted Grafana credentials), `kustomization.yaml` (Helm chart inclusion + KSOPS), and `application.yaml` (ArgoCD Application CRD).

- **IngressClass migration**: Changed all ingress definitions from `ingressClassName: traefik` to `ingressClassName: cilium` in the Helm values to consolidate on Cilium ingress controller.

- **Dynamic certificate management**: Integrated existing `etcd-cert-syncer` CronJob into the ArgoCD Application by adding it as a resource in `kustomization.yaml`. The CronJob runs daily to refresh certificates from control plane nodes (`/etc/kubernetes/pki/etcd/`) into the monitoring namespace.

- **Bootstrap automation**: Added a one-time Job (`etcd-cert-syncer-bootstrap`) alongside the CronJob to handle initial secret creation. Used ArgoCD sync waves (`argocd.argoproj.io/sync-wave: "0"` for bootstrap, `"1"` for Prometheus) to ensure proper ordering. Job includes `ttlSecondsAfterFinished: 86400` for automatic cleanup.

- **Clean Helm handoff**: Used `helm uninstall kube-prometheus-stack --namespace monitoring --keep-history` to remove Helm ownership, then let ArgoCD sync recreate all resources from GitOps definitions. This eliminated dual ownership conflicts.

**Why it took the time it took**: This migration touched multiple complex systems simultaneously. The bootstrap dependency issue only became apparent during testing when we discovered that deleting the secret caused Prometheus to fail startup. The standard CronJob approach worked for ongoing operations but created a manual initialization gap. The sync wave solution required understanding ArgoCD's resource ordering capabilities and testing the proper annotations in both Job metadata and Helm chart values.

**Prevention going forward**:

- **Bootstrap pattern documented**: Any component requiring external dependencies (certificates, external secrets, etc.) should include both ongoing automation (CronJob) and bootstrap automation (one-time Job with sync waves) in the same ArgoCD Application.

- **Helm migration checklist**: (1) Extract current Helm values, (2) Split into non-sensitive (`values.yaml`) and sensitive (`secret.enc.yaml`) portions, (3) Create ArgoCD Application with manual sync initially, (4) Test deployment, (5) Remove Helm release with `--keep-history`, (6) Let ArgoCD recreate resources, (7) Enable auto-sync once stable.

- **Dependency ordering**: Use ArgoCD sync waves for any components with initialization dependencies. Bootstrap components should be `sync-wave: "0"`, dependents should be higher numbers. Document the dependency chain in the Application or values file.

- **Certificate management pattern**: External certificate sync should always include both scheduled refresh (CronJob) and bootstrap initialization (Job). Both should be managed within the same ArgoCD Application as the consuming workload to maintain GitOps coherence.

**References**:
- Initial migration: `58bf457` (platform/monitoring: migrate kube-prometheus-stack to ArgoCD)  
- CronJob integration: `b98794b` (platform/monitoring: add etcd-cert-syncer CronJob to kube-prometheus-stack)
- Bootstrap automation: `bb3503d` (platform/monitoring: add bootstrap Job for etcd-client-certs)
- ArgoCD sync waves docs: <https://argo-cd.readthedocs.io/en/stable/user-guide/sync-waves/>
- Kustomize Helm integration: <https://kubectl.docs.kubernetes.io/references/kustomize/builtins/#helmchartinflationgenerator>

---

## 2026-04-28 — ksops CMP discovery timeout

**What happened**: While migrating `apps/homepage/` from raw `kubectl apply` to ArgoCD-managed (commit `59d0984`), the new `homepage` Application stayed stuck in `ComparisonError` with:

> `Failed to load target state: failed to generate manifest for source 1 of 1: rpc error: code = Unknown desc = Manifest generation error (cached): CMP processing failed for application "homepage": could not find cmp-server plugin with name "ksops" supporting the given repository`

The same `plugin: { name: ksops }` Application config worked fine for every existing component under `platform/` (promtail, external-dns, …). The path was correct. The `kustomization.yaml` was present and named exactly as the plugin's `discover.fileName: ./kustomization.yaml` required. Manually rendering `kubectl kustomize apps/homepage/` (after temporarily moving `ksops.yaml` aside, since plugins are blocked locally) produced the expected manifests with image pin and `envFrom` intact.

**Root cause**: not a missing plugin, not a discovery rule mismatch. The KSOPS sidecar's `MatchRepository` gRPC was timing out at exactly 60s on every call:

```
{"level":"error","grpc.code":"Unknown",
 "grpc.error":"match repository error receiving stream: error receiving tgz file:
                stream context error: context deadline exceeded",
 "grpc.method":"MatchRepository","grpc.time_ms":"59404.977"}
```

Discovery in CMP works by the repo-server **streaming the entire working tree as a tarball** to the sidecar before any filename filter is applied. Our working tree contained ~46 MB of non-ArgoCD content:

- `infrastructure/kubernetes/installation/` — ~29 MB of manual kubeadm install scripts and configs.
- `apps/chat-app/flask-service/static/` — ~16 MB of static web assets.
- Smaller misc under `automation/`.

That tarball cannot stream through the in-pod UDS within the 60s deadline on a cold MatchRepository. The error message — *"could not find cmp-server plugin with name ksops"* — is misleading: ArgoCD presents discovery timeouts as if the plugin doesn't exist. The error gets cached (`(cached)` in the message) so subsequent reconciles keep showing it even though the underlying transfer is not retried until the cache invalidates.

The `platform/` Applications worked because they were already cached from a prior successful generation when the working tree was smaller. New Applications (or any cold cache) hit the failure.

**Fix** (commit `48630e6`):

- Added [`/.argocdignore`](../../.argocdignore) excluding `infrastructure/`, `apps/chat-app/`, `automation/`. Same syntax as `.gitignore`. Recognized by argocd-repo-server when assembling the tarballs sent to CMP plugins. Cuts the discovery payload from ~46 MB to <1 MB.
- After pushing, hard-refreshed the cached error: `argocd app refresh homepage --hard` (or `kubectl -n argocd delete pod -l app.kubernetes.io/name=argocd-redis-ha-haproxy` as a heavier hammer; we used the former).

**Why it took the time it took**: the error message points at the plugin, not at the transport. Three false leads were easy to fall into:

1. *"The plugin isn't installed in the repo-server pod."* It is — `kubectl exec ... -c ksops -- cat /home/argocd/cmp-server/config/plugin.yaml` showed the correct config.
2. *"The discover rule doesn't match."* It does — `./kustomization.yaml` is the literal filename in our directory.
3. *"It's the path."* It isn't — `apps/homepage` is no different from `platform/<x>` to ArgoCD.

The smoking gun was in the `ksops` sidecar logs, not in the Application status: every `MatchRepository` call ended in `context deadline exceeded` at ~59 seconds, with `error receiving tgz file`. The "tgz" term is the giveaway — once you see it, you go look at what's in the tree.

**Prevention going forward**:

- New ARCHITECTURE doc: [`platform/argocd/ARCHITECTURE.md`](./ARCHITECTURE.md) §4–7 explain the Match-vs-Generate distinction, the 60s deadline, and the role of `.argocdignore`. Anyone hitting the same symptom should now find it in two clicks from `platform/CLAUDE.md`.
- `.argocdignore` needs maintenance: when adding a new top-level directory that *is not* an ArgoCD source path, add it to `.argocdignore` in the same commit. The file itself documents the rule.
- The diagnostic trick: if a plugin error mentions `(cached)` and looks impossible, the next step is *always* `kubectl logs -n argocd <repo-server-pod> -c <plugin-sidecar>` and look for `time_ms` ≈ `59000`. If you see that, it's a transport timeout.

**References**:
- Migration commit: `59d0984` (apps/homepage migrated to ArgoCD-managed).
- Fix commit: `48630e6` (`.argocdignore` added).
- Relevant ArgoCD docs: <https://argo-cd.readthedocs.io/en/stable/operator-manual/high_availability/#argocd-repo-server> and <https://argo-cd.readthedocs.io/en/stable/operator-manual/config-management-plugins/>.
- KSOPS upstream: <https://github.com/viaduct-ai/kustomize-sops>.
