# 2026-05-02 — Cluster-wide RBAC wildcard deletion

**Severity:** S1 (cluster-control-plane impact, recovered).
**Duration:** ~2h from trigger to full recovery of critical-path components; tail-cleanup of non-critical RBAC tracked separately.
**Window:** ~02:11 UTC (delete) → ~04:00 UTC (cilium + argocd helm re-applies green).

---

## Summary

A `kubectl delete` command issued during the `monitoring → observability` namespace migration (Phase B0.3 cleanup) included `clusterrole,clusterrolebinding` in the kind list with the `--all` flag and a `-n monitoring` namespace selector. `-n` is silently ignored for cluster-scoped kinds, so `--all` matched **every ClusterRole in the cluster**. The delete iterated alphabetically and removed ~33 ClusterRoles before it reached `cluster-admin` — at which point the issuing identity (`system:serviceaccount:kube-system:admin-sa`, bound to cluster-admin via a ClusterRoleBinding) lost privilege and all subsequent deletes returned `403 Forbidden: cluster-admin not found`. The delete halted partway through the alphabet, so impact was bounded to roles whose names sort before `cluster-admin`.

---

## Trigger

Force-cleanup of resources stuck in the `monitoring` namespace during the namespace migration. `monitoring` was Terminating but its StatefulSets/Services/CMs/Secrets/PVCs hadn't been pruned because the kube-prometheus-stack ArgoCD Application has `prune: false`. To unblock the namespace finalizer the operator ran:

```bash
kubectl delete cm,secret,sa,role,rolebinding,clusterrole,clusterrolebinding,servicemonitor,podmonitor,prometheusrule -n monitoring --all
```

The intent was to delete every namespaced object in `monitoring`. The bug was bundling `clusterrole` and `clusterrolebinding` (both cluster-scoped) into the same kind list. `kubectl` evaluated `--all` against each kind independently and ignored `-n monitoring` for cluster-scoped kinds — exactly as documented, just easy to miss when the kind list is long.

---

## Detection

Immediate. The same `kubectl delete` produced `403 Forbidden: ... cluster-admin not found` on its later list-and-delete passes (e.g. for `clusterroles "view"`, `clusterroles "system:volume-scheduler"`). Subsequent `kubectl get clusterrole` calls failed with the same error. The user's working `KUBECONFIG` (`admin-sa` ServiceAccount token) was de-privileged because its CRB pointed at the now-missing `cluster-admin` ClusterRole.

Downstream symptoms surfaced over the next ~30 min:
- ArgoCD application-controller pods on certain worker nodes started failing with `dial tcp 10.96.118.120:8081: connect: no route to host` (repo-server's Service IP, no routes from those pods).
- New pods scheduled to any worker stayed in `cilium-endpoint identity=5 (reserved:init)` because the cilium-operator pod on `master-01` was CrashLoopBackOff with `RBAC: clusterrole.rbac.authorization.k8s.io "cilium-operator" not found` — couldn't acquire its leader-election lease.

---

## Root cause

Two contributing factors:

1. **`kubectl delete <kinds> -n <ns> --all` semantics.** When the kind list mixes namespaced and cluster-scoped kinds, the command silently drops the namespace filter for cluster-scoped resources and wildcard-deletes everything cluster-wide. There is no warning. There is no confirmation. (`kubectl delete --all-namespaces` exists and is loud; `--all` against cluster-scoped kinds when `-n` is set is silent and dangerous.)

2. **No protection on system-critical ClusterRoles.** `cluster-admin`, `cilium`, `cilium-operator`, `argocd-application-controller`, etc. are deletable by anyone with `delete clusterroles` permission. Kubernetes does not protect bootstrap-flagged ClusterRoles from deletion (the `rbac.authorization.kubernetes.io/autoupdate` annotation only governs *re-creation* on controller-manager startup, not *deletion*).

The first is operator error; the second is a system property of how RBAC is implemented and worth knowing.

---

## Impact

### Critical-path (recovered 2026-05-02)
| Component | Symptom | Recovery |
|---|---|---|
| `cluster-admin` ClusterRole | All admin-sa kubectl calls 403 | Manual recreate via system:masters rescue cert; kube-controller-manager bootstrap controller union'd in the missing `nonResourceURLs` rule on next pass |
| `cilium`, `cilium-operator` ClusterRoles | New pods stuck in `reserved:init` identity → no egress; CrashLoopBackOff on `cilium-operator` (couldn't acquire lease) | `helm upgrade cilium cilium/cilium --version 1.18.0` (chart re-applies missing CRs) |
| `argocd-application-controller`, `argocd-server`, `argocd-notifications-controller` ClusterRoles | App-controller pod failed liveness; CMP/repo-server requests timed out from controller pods | `helm upgrade --install argocd argo-cd --version 9.5.4` |

### Deferred recovery (tracked as GH issue #TBD)
| Component group | Roles missing | Source | Risk if left alone |
|---|---|---|---|
| `cert-manager-*` (9 roles) | 9 | ArgoCD-managed (`platform/cert-manager/`) | Cert renewal halts when the cached watch state expires; impacts new TLS certs and renewals on the ~60-day Let's Encrypt cycle |
| `ceph-csi-*`, `cephfs-*` (8 roles) | 8 | rook-ceph helm release | New PVC provisioning and node-plugin operations; existing volumes keep mounting via their cached attachment state |
| `capi-*`, `capmox-*`, `capo-*` (10+ roles) | 10+ | clusterctl install | Only matters when provisioning new CAPI machines; cluster components installed but not actively used today |

### Not impacted
- All ClusterRoles whose names sort lexically *after* `cluster-admin`. Kubernetes system roles (`system:*`), `view`, `edit`, `admin`, etc. — all preserved.
- All workloads currently running. Existing pods kept their cached watch state; only operations that re-validated RBAC (new pod creation needing identity, leader-election renewal, fresh API watches after pod restart) failed.

---

## Recovery timeline (UTC)

- **02:11** — Wildcard delete fired. Bounded by RBAC failure once `cluster-admin` was hit.
- **02:13** — `cluster-admin not found` errors detected. `kubectl auth can-i` returns no for everything.
- **02:30** — Confirmed root cause via investigation of the delete-command's effect; identified that the issuing kubeconfig used a SA token bound to the deleted role.
- **02:50** — `super-admin.conf` not present (cluster bootstrapped pre-1.29 kubeadm; kubeconfig split not auto-backfilled on upgrade). Generated a one-shot rescue client cert in `O=system:masters` group, signed by cluster CA.
- **03:00** — Recreated `cluster-admin` ClusterRole using rescue kubeconfig. Restarted kube-controller-manager. Bootstrap controller subsequently union'd in the missing `nonResourceURLs` rule.
- **03:30** — Inventory: 33 ClusterRoles missing (every role whose name sorts before `cluster-admin`).
- **03:40** — Detected ArgoCD app-controller "no route to host" symptoms. Initially diagnosed as Cilium ebpf staleness; restart attempted.
- **03:50** — Identified the new app-controller pod stuck in cilium `reserved:init` identity. cilium-operator CrashLoopBackOff log named the actual cause: missing `cilium-operator` ClusterRole.
- **03:57** — `helm upgrade cilium`. ClusterRoles back. Operator pod recovered. New pods got real cilium identities.
- **04:00** — `helm upgrade argocd`. ClusterRoles back. Application-controller Ready.
- **04:15** — Critical path verified. RCA + GH issue drafted.

---

## What worked

- **Defense-in-depth on the rescue path.** kubeadm 1.29+ provides `super-admin.conf` for exactly this scenario; even though the cluster didn't have one (old bootstrap), a manual cert in `O=system:masters` was a 30-line openssl block away.
- **Existing helm-managed components.** Cilium and ArgoCD are documented as helm-managed (`platform/CLAUDE.md` lists them as exceptions to the ArgoCD-managed default). A single `helm upgrade` re-applied each chart's full RBAC bundle including the missing ClusterRoles. No GitOps recovery needed for the most critical pieces.
- **Bounded blast radius.** The delete halted itself once it hit `cluster-admin` and broke the issuing identity. This was lucky — had it been issued by a more privileged identity (system:masters, or a non-RBAC-bound auth), the delete would have continued through the entire ClusterRole list.

## What didn't work

- **The first attempt to use the existing `admin.conf` for recovery failed**, because on this cluster (originally bootstrapped pre-1.29, upgraded to 1.34) `admin.conf` authenticates as `O=kubeadm:cluster-admins` — which is itself bound to `cluster-admin` via a ClusterRoleBinding. With `cluster-admin` deleted, the binding was dead and `admin.conf` was no more privileged than any other client. **`super-admin.conf` was not auto-generated during the upgrade**, so the canonical "RBAC nuked, use super-admin" recovery path didn't exist on this cluster.
- **Multi-line shell pastes corrupted recovery commands.** The terminal's auto-indent / line-wrap mangled heredocs and long base64 strings several times during the recovery, requiring multiple restarts of cert generation. Using only single-line `kubectl config set-*` commands worked reliably.

---

## Action items

| Item | Owner | Status |
|---|---|---|
| Recover deferred RBAC (cert-manager, rook-ceph CSI, CAPI) | operator | tracked in GH issue (link below) |
| Generate `super-admin.conf` proactively (`kubeadm init phase kubeconfig super-admin`) so future RBAC incidents have a one-step rescue | operator | done 2026-05-02 |
| Document the kubectl delete footgun in `platform/CLAUDE.md` "Common pitfalls" | next session | pending |
| Add a Kyverno or admission-webhook policy that protects bootstrap-managed ClusterRoles from deletion (defense in depth) | next session — fits the security roadmap (Layer 3) | pending |
| Rotate `admin-sa` token (precaution; the token wasn't exfiltrated but did briefly hit a degraded cluster) | optional | pending |
| Replace bulk `kubectl delete <kinds> --all` patterns in operator runbooks with explicit, namespaced manifests or per-resource deletion | next session | pending |

---

## Lessons

1. **Never `kubectl delete <kinds> -n <ns> --all` when the kind list contains *any* cluster-scoped kind.** The `-n` filter is silently dropped for cluster-scoped kinds. Use one command per scope, or `kubectl delete ns <name>` (which cascades safely through the namespace's owned resources only).

2. **`super-admin.conf` is the rescue credential of choice on 1.29+ clusters.** Generate it once (`sudo kubeadm init phase kubeconfig super-admin`) and back it up; without it, RBAC recovery requires hand-rolled openssl cert generation against the cluster CA — doable but slow.

3. **`autoupdate: "true"` annotation re-creates rules but not rules-only-after-delete-and-recreate.** The kube-controller-manager RBAC bootstrap controller will reconcile *bootstrap* ClusterRoles (cluster-admin, system:*) on its next leader's startup. Third-party ClusterRoles (cilium, argocd, cert-manager, …) are not bootstrap-managed; once deleted, they require their owner's installer (helm chart, operator, etc.) to re-apply.

4. **Distributed ClusterRole damage cascades silently** because pods retain their cached API watches. The cluster appears "mostly fine" right after the deletion; failures emerge only when something needs to re-establish auth (new pod, lease renewal, watch restart). This makes the impact harder to assess in real time.

---

## References

- Kubernetes RBAC bootstrap policy: <https://github.com/kubernetes/kubernetes/blob/master/plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go>
- kubeadm `super-admin.conf` introduction: KEP-4128 / <https://kubernetes.io/blog/2024/04/17/kubernetes-1-30-release-announcement/#new-conformance-mode-for-validateadmissionpolicy>
- Cilium identity allocation (`reserved:init` semantics): <https://docs.cilium.io/en/stable/concepts/security/identity/>
