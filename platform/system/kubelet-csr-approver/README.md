# kubelet-csr-approver

ArgoCD Application for [kubelet-serving CSR auto-approver](https://github.com/TruongHoangPhuLoc/kubelet-serving-csr-approver).

A single-purpose controller that auto-approves
`kubernetes.io/kubelet-serving` CertificateSigningRequests against a strict
12-rule policy. Removes the manual `kubectl certificate approve` toil while
preserving the safety property that no kubelet can request a serving cert
with SANs that don't match its real identity.

## Component shape

Raw-manifest pattern — same as `apps/homepage/`. There is no helm chart;
manifests live in the controller repo's `deploy/` directory and are pulled
in as a remote kustomize base. This overlay only contains:

- `application.yaml` — the ArgoCD Application spec.
- `kustomization.yaml` — the remote base reference + image tag override.
- This README.

## Prerequisite: enable `serverTLSBootstrap` on kubelets

**Without this, the controller has nothing to do.** By default, kubelets
generate a self-signed serving certificate at startup and never submit a
CSR. To make them use the CSR flow:

1. Add to every node's kubelet config (typically `/var/lib/kubelet/config.yaml`):

   ```yaml
   serverTLSBootstrap: true
   rotateCertificates: true   # already on by default
   ```

2. Restart the kubelet on each node: `systemctl restart kubelet`.

3. Each kubelet will submit a `kubernetes.io/kubelet-serving` CSR within
   ~5s of restart. The first round you'll see them all queue up as
   `Pending`. Once this Application is healthy, the controller approves
   them; kube-controller-manager signs; kubelet writes the new cert.

If you set this **after** the Application is healthy, expect a brief
storm of pending CSRs followed by approvals as nodes restart.

## Verifying it works

```bash
kubectl --context home-cluster -n kube-system get pods -l app.kubernetes.io/name=kubelet-csr-approver
kubectl --context home-cluster -n kube-system logs -l app.kubernetes.io/name=kubelet-csr-approver

# List recent kubelet-serving CSRs and their decision conditions:
kubectl --context home-cluster get csr \
  -o custom-columns='NAME:.metadata.name,SIGNER:.spec.signerName,USER:.spec.username,CONDS:.status.conditions[*].type' \
  | grep kubelet-serving
```

A working state looks like every `kubelet-serving` CSR ending in `Approved`
or `Approved,Issued`. A `Denied` is the controller working as designed
(check the controller logs for the rule that fired — names from
[`internal/policy/policy.go`](https://github.com/TruongHoangPhuLoc/kubelet-serving-csr-approver/blob/main/internal/policy/policy.go)).

## Upgrading

Two knobs in `kustomization.yaml`:

- **`resources: …?ref=…`** — bumps the manifests version (Deployment shape,
  RBAC, etc.). Use a tag once one exists; `main` for now.
- **`images.newTag`** — bumps the container image only. After each green CI
  run on the controller repo, copy the `sha-<short>` tag from
  `ghcr.io/truonghoangphuloc/kubelet-serving-csr-approver` here and commit.

For most rollouts only the image tag changes.

## Promoting sync policy

This Application starts with `prune: false, selfHeal: false` per
[platform/CLAUDE.md](../CLAUDE.md) rule 5. After 2–3 clean sync cycles with
no unexpected drift, flip both to `true` **in a separate commit** — keeps
the promotion explicit and easy to revert.

## RBAC footprint

Minimum-RBAC by design. See `deploy/clusterrole.yaml` in the controller
repo for the exact rules. Summary:

- `nodes`: get / list / watch (read addresses for SAN allowlist)
- `certificatesigningrequests`: get / list / watch (read CSRs)
- `certificatesigningrequests/approval`: update (apply decision)
- `signers` resourceName `kubernetes.io/kubelet-serving`: approve
  (authorize approval *only* for this signer)

Anything broader than this is a bug — flag and fix.

## ArgoCD project

This Application is scoped to the `system` AppProject (defined in
[`../project.yaml`](../project.yaml)). The project guardrails:

- Destination is locked to `kube-system`. A misconfigured
  `destination.namespace` shows up as OutOfSync instead of leaking
  resources into another namespace.
- ClusterRole / ClusterRoleBinding installs are permitted (the controller
  legitimately needs cluster-scoped RBAC for CSRs and signers).

Apply the project once before the first Application sync:

```bash
kubectl --context home-cluster apply -f platform/system/project.yaml
```
