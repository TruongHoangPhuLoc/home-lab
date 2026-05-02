# CLAUDE.md

Instructions for AI agents (and humans) working in this repo. Root-level rules only; specific conventions live alongside the code they govern — follow pointers below.

## Project in one paragraph

Personal homelab. Proxmox-hosted VMs + a self-provisioned Kubernetes cluster + supporting platform/observability/automation stack. ArgoCD (v3.3.8) manages most cluster workloads via a Kustomize-wraps-Helm + SOPS/KSOPS pattern — details in [`platform/CLAUDE.md`](./platform/CLAUDE.md). Secrets are SOPS-encrypted with age and committed as `*.enc.yaml` files — full guide in [`platform/argocd/BOOTSTRAP.md`](./platform/argocd/BOOTSTRAP.md).

## Top-level layout

| Directory | Purpose | Specific rules |
|---|---|---|
| `apps/` | End-user workloads (chat-app, homepage, …) | — (future `apps/CLAUDE.md`) |
| `platform/` | Everything cluster-wide that supports workloads: argocd, cert-manager, cluster-api, ingress, networking, observability, storage. **ArgoCD-managed by default.** | [`platform/CLAUDE.md`](./platform/CLAUDE.md) |
| `infrastructure/` | Bare-metal / VM layer: proxmox, networking (OPNsense/BIND/HAProxy), docker services, kubernetes install scripts | — (future `infrastructure/CLAUDE.md`) |
| `automation/` | Config management + CI/CD: ansible, jenkins | — (future `automation/CLAUDE.md`) |
| `incidents/` | Post-mortems for cluster-wide incidents. One file per event. | [`incidents/README.md`](./incidents/README.md) |

## Global rules

These apply everywhere in the repo.

1. **No plaintext secrets in git, ever.** All sensitive data lives in `*.enc.yaml` files, encrypted with SOPS using age. Config: [`.sops.yaml`](./.sops.yaml). Workflow: [`platform/argocd/BOOTSTRAP.md`](./platform/argocd/BOOTSTRAP.md). A pre-commit hook + GitHub Actions scan will eventually enforce this — currently relies on operator discipline.
2. **No vendored helm charts.** Charts are pulled from upstream repos at render time; this repo only holds thin config overlays (values, secrets, Application manifests).
3. **No ArgoCD Applications created via the UI.** Every Application is defined by a committed `application.yaml`. UI-created Applications disappear when the cluster is rebuilt.
4. **Commit messages explain *why*.** Cite impacted systems. Note any manual steps the operator must run alongside the commit (e.g., "before applying this, run: `kubectl create secret …`").

## Destructive-operation rules

Hard-learned from real incidents (see [`incidents/`](./incidents/)). Treat as non-negotiable.

5. **Never `kubectl delete <kinds> -n <ns> --all` when the kind list contains a cluster-scoped kind.** The `-n` filter is silently dropped for cluster-scoped resources, and `--all` will match cluster-wide. Bug fixed an entire afternoon's worth of cleanup into a 33-ClusterRole loss in the [2026-05-02 RBAC wildcard deletion](./incidents/2026-05-02-rbac-wildcard-deletion.md). For namespace cleanup, use `kubectl delete ns <name>` — it cascades through the namespace's owned resources only and stops at the boundary. For deletion of cluster-scoped resources, name them explicitly, never with `--all`.
6. **`super-admin.conf` is the rescue credential.** kubeadm 1.29+ generates it at init time; older clusters need `sudo kubeadm init phase kubeconfig super-admin` once, manually. Without it, recovery from RBAC damage requires hand-rolled openssl client-cert generation against the cluster CA. Verify it exists on every control-plane node and back it up offline.
7. **Identities bound to `cluster-admin` evaporate when `cluster-admin` is deleted.** This includes the kubeadm-generated `admin.conf` on 1.29+ clusters (which authenticates as `O=kubeadm:cluster-admins`, bound to cluster-admin via a CRB). Don't assume admin.conf is a "rescue" credential — only `super-admin.conf` (`O=system:masters`) bypasses RBAC entirely.
8. **Helm-managed components own their RBAC.** When third-party ClusterRoles disappear (cilium, argocd, cert-manager, rook-ceph-csi, …), recovery is `helm upgrade <release> ... -f values.yaml` — re-applies the chart's full RBAC bundle without disturbing existing resources. ArgoCD-managed components recover via a force-sync of the Application once ArgoCD itself is back. Default ClusterRoles (`cluster-admin`, `system:*`) are auto-recreated by kube-controller-manager's RBAC bootstrap controller on its leader's startup, *if* the role's `autoupdate: "true"` annotation is preserved.

## Secrets workflow (quick reference)

```bash
export SOPS_AGE_KEY_FILE="$HOME/.config/sops/age/keys.txt"    # put in ~/.bashrc

sops new.enc.yaml              # create + edit ($EDITOR opens plaintext)
sops existing.enc.yaml         # edit in place (decrypt → edit → re-encrypt)
sops -d file.enc.yaml          # print decrypted to stdout
sops updatekeys file.enc.yaml  # re-encrypt after .sops.yaml changes
```

Public key for this repo (safe to share): `age1f2ga2qhdv6hpfhlelk7t633yzh78u4jdkwxkxrcpml5a7tzyd9ps99zmkj`

## Where to find specific rules

| Topic | File |
|---|---|
| ArgoCD Applications, Kustomize-wraps-Helm+KSOPS pattern, helm-managed exceptions (cilium, rook-ceph) | [`platform/CLAUDE.md`](./platform/CLAUDE.md) |
| SOPS setup, key rotation, recovery procedures, planned safety checks | [`platform/argocd/BOOTSTRAP.md`](./platform/argocd/BOOTSTRAP.md) |
| Cluster-wide incident reports + RBAC / cluster-recovery runbooks | [`incidents/`](./incidents/) |
| Repo overview, achievements, current direction | [`README.md`](./README.md) |

## Conventions for agents

- When adding a CLAUDE.md to a new subdirectory, follow the same pattern: start with the scope ("rules for everything under X"), state invariants, provide canonical templates, list exceptions explicitly.
- If you find yourself repeating a rule in multiple places, that's a signal it belongs in this root file.
- Prefer to extend existing memory (`~/.claude/projects/.../memory/*.md`) over adding a new one when the topic overlaps.
