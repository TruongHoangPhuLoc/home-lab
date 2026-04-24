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

## Global rules

These apply everywhere in the repo.

1. **No plaintext secrets in git, ever.** All sensitive data lives in `*.enc.yaml` files, encrypted with SOPS using age. Config: [`.sops.yaml`](./.sops.yaml). Workflow: [`platform/argocd/BOOTSTRAP.md`](./platform/argocd/BOOTSTRAP.md). A pre-commit hook + GitHub Actions scan will eventually enforce this — currently relies on operator discipline.
2. **No vendored helm charts.** Charts are pulled from upstream repos at render time; this repo only holds thin config overlays (values, secrets, Application manifests).
3. **No ArgoCD Applications created via the UI.** Every Application is defined by a committed `application.yaml`. UI-created Applications disappear when the cluster is rebuilt.
4. **Commit messages explain *why*.** Cite impacted systems. Note any manual steps the operator must run alongside the commit (e.g., "before applying this, run: `kubectl create secret …`").

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
| Repo overview, achievements, current direction | [`README.md`](./README.md) |

## Conventions for agents

- When adding a CLAUDE.md to a new subdirectory, follow the same pattern: start with the scope ("rules for everything under X"), state invariants, provide canonical templates, list exceptions explicitly.
- If you find yourself repeating a rule in multiple places, that's a signal it belongs in this root file.
- Prefer to extend existing memory (`~/.claude/projects/.../memory/*.md`) over adding a new one when the topic overlaps.
