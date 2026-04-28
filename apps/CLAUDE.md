# apps/CLAUDE.md

Rules for everything under `apps/`. Read this *in addition to* the root [`CLAUDE.md`](../CLAUDE.md) when adding or modifying end-user workloads in this subtree. Many conventions mirror [`platform/CLAUDE.md`](../platform/CLAUDE.md) — when in doubt, follow it.

## Scope

`apps/` holds **end-user workloads** (dashboards, services I built or run for myself). It is *not* for cluster infrastructure — that lives in [`platform/`](../platform/). Examples:

| App | What it is |
|---|---|
| `homepage/` | gethomepage/homepage dashboard |
| `chat-app/` | self-built Flask + websocket demo (not yet ArgoCD-managed) |

## Invariant

**Apps are ArgoCD-managed by default.** Same Application/Kustomize+KSOPS pattern as `platform/`. Two pattern variants exist (pick by chart availability — see below).

## The two component-directory shapes

Pick **one** based on whether the app has a maintained upstream helm chart:

### Variant A — Kustomize-wraps-Helm (preferred when an official chart exists)

Identical to the platform pattern. See [`platform/CLAUDE.md`](../platform/CLAUDE.md#the-canonical-component-directory) for the canonical templates.

```
apps/<app>/
├── application.yaml      # ArgoCD Application CRD
├── kustomization.yaml    # helmCharts: + (optional) generators
├── values.yaml           # helm overrides, plaintext, non-sensitive only
├── ksops.yaml            # OPTIONAL — only when secret.enc.yaml exists
└── secret.enc.yaml       # OPTIONAL — SOPS-encrypted Secret(s)
```

### Variant B — Raw manifests + Kustomize + KSOPS (when no chart exists)

Used when there is no maintained official helm chart and writing one (or trusting an unofficial one) is more risk than value. Resources are committed directly. Reference: [`apps/homepage/`](./homepage/).

```
apps/<app>/
├── application.yaml      # ArgoCD Application CRD (plugin: ksops still applies — it renders kustomize)
├── kustomization.yaml    # resources: + generators: (no helmCharts)
├── manifests.yaml        # most/all resources (SA, RBAC, Deployment, Service, Ingress, …)
├── configmap.yaml        # SEPARATE file when the app's config is large/edited often
├── ksops.yaml            # OPTIONAL — only when secret.enc.yaml exists
└── secret.enc.yaml       # OPTIONAL — SOPS-encrypted Secret(s)
```

**When to choose Variant B:**
- The upstream project explicitly declines to maintain a helm chart (e.g. gethomepage — see https://github.com/gethomepage/homepage/discussions/5874).
- The app is something we built ourselves (chat-app).

**Don't choose Variant B just to "keep things simple."** Helm charts give us upgrade paths, defaults, and easier diffs against upstream — prefer Variant A whenever a maintained chart exists.

### Canonical templates (Variant B specifics)

Only the bits that differ from `platform/CLAUDE.md` are shown here.

**`kustomization.yaml`**
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - manifests.yaml
  - configmap.yaml         # if you split it out

# Add this block ONLY if the component has secrets:
# generators:
#   - ./ksops.yaml
```

**`application.yaml`** — same template as platform, with `path: apps/<app>` and `plugin: { name: ksops }`. The `plugin: ksops` setting renders kustomize (with or without encrypted files); KSOPS files only get decrypted if `ksops.yaml` references them.

## Non-negotiable rules (additions to platform's)

All [eight platform non-negotiables](../platform/CLAUDE.md#non-negotiable-rules) apply here verbatim. In particular: pin image tags explicitly (no `:latest`), secrets only in `secret.enc.yaml`, `ServerSideApply=true`, and `Application.metadata.name` matches the app's directory name.

Apps-specific:

9. **Pin container image tags.** End-user apps with `image: foo:latest + imagePullPolicy: Always` are the most common cause of mysterious post-redeploy regressions. Pin to a semver tag.
10. **Config-as-env-var-substitution for secrets.** When an app's config file (e.g., `services.yaml` for homepage) inlines credentials, do **not** put them in `values.yaml` or the ConfigMap. Use the app's environment-variable substitution feature and source the values via `envFrom: secretRef` from a SOPS-encrypted Secret.

### Editing `*.enc.yaml` (don't let plaintext land on disk)

The committed blob must always contain `ENC[...]` ciphertext under `stringData` / `data` (verify with `grep '^sops:'`). If your working copy shows bare quoted secrets while `git show HEAD:<path>` still shows ciphertext, someone (or an editor autosave) **wrote decrypted YAML over** the encrypted file — common after `sops -d file.enc.yaml > file.enc.yaml` by mistake or opening the file outside `sops`. **Don't commit.** Restore the last encrypted revision, then edit only via SOPS:

```bash
git restore apps/<app>/secret.enc.yaml
export SOPS_AGE_KEY_FILE="$HOME/.config/sops/age/keys.txt"
sops apps/<app>/secret.enc.yaml
```

Cursor / VS Code Git diffs comparing your plaintext working tree against `HEAD` will look alarming; after `git restore`, the diff disappears.

## Adoption workflow (raw-manifest case, Variant B)

The full helm-release adoption workflow is in [`platform/CLAUDE.md`](../platform/CLAUDE.md#adoption-workflow--migrate-an-existing-helm-release-to-argocd). For Variant B (the app was applied as raw manifests, no helm release ever existed), the only differences are steps 1–2:

1. **Pull the live state from the cluster** for everything that's drifted from git:
   ```bash
   kubectl --context home-cluster -n <ns> get configmap <name> -o yaml > /tmp/<name>-cm.yaml
   kubectl --context home-cluster -n <ns> get deployment <name> -o yaml > /tmp/<name>-dep.yaml
   ```
2. **Sanitize and split:**
   - Strip cluster-managed fields (`status`, `metadata.{resourceVersion,uid,creationTimestamp,generation,managedFields,annotations[kubectl.kubernetes.io/last-applied-configuration]}`).
   - Move every credential out of plaintext into `secret.enc.yaml` (HOMEPAGE_VAR_-style or the app's equivalent).
   - Pin image tags.

Steps 3–9 (write files, commit, clean up old broken Application, apply new Application, verify, promote sync policy if desired, stop running `kubectl apply` from this point on) are the same as platform's.

## Currently-managed apps

| App | Variant | Notes |
|---|---|---|
| [`homepage/`](./homepage/) | B (raw manifests) | Migrated 2026-04-28. No official helm chart exists. Image pinned to `v1.12.3`. Credentials in `secret.enc.yaml` consumed via `{{HOMEPAGE_VAR_*}}` substitution. |
| [`chat-app/`](./chat-app/) | not yet ArgoCD-managed | Raw YAML in `chat-app/k8s/`. Migrate when ready. |

## When in doubt

1. Open [`apps/homepage/`](./homepage/) as the Variant B reference, or any `platform/<category>/<component>/` for Variant A.
2. Re-read [`platform/CLAUDE.md`](../platform/CLAUDE.md) — most rules carry over verbatim.
3. Prefer asking before taking a destructive cluster action. Adoption should be idempotent — if it feels risky, it usually is.
