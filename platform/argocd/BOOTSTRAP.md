# ArgoCD + SOPS bootstrap

Secret management in this repo uses [SOPS](https://github.com/getsops/sops) with [age](https://github.com/FiloSottile/age) encryption. Encrypted files live in git as `*.enc.yaml`; a KSOPS plugin runs as a sidecar in `argocd-repo-server` and decrypts them at render time before ArgoCD applies manifests to the cluster.

Configuration lives in [`/.sops.yaml`](../../.sops.yaml) at the repo root.

## Argo CD Helm overrides

The Argo CD release itself is installed with Helm. User-supplied values are committed as [`helm-values.yaml`](./helm-values.yaml). After editing that file, apply with:

```bash
helm upgrade argocd argo-cd \
  --repo https://argoproj.github.io/argo-helm \
  --version 9.5.4 \
  -n argocd \
  -f platform/argocd/helm-values.yaml
```

Pin `--version` to the chart revision shown by `helm list -n argocd` when you upgrade across chart bumps. Resource **requests** on `repo-server`, `ksops`, and `server` containers are required so the built-in HPAs can compute CPU/memory utilization (otherwise metrics stay `<unknown>` and scaling never activates).

## Keys

### Public key (safe to commit)

```
age1f2ga2qhdv6hpfhlelk7t633yzh78u4jdkwxkxrcpml5a7tzyd9ps99zmkj
```

Used to encrypt. Embedded in `.sops.yaml`.

### Private key (do NOT commit)

- **Location on operator workstation:** `~/.config/sops/age/keys.txt` (mode 600)
- **Backup:** password manager (mandatory — if both the local file and the backup are lost, every `.enc.yaml` in this repo is permanently unrecoverable).
- **Shell integration:** export `SOPS_AGE_KEY_FILE=$HOME/.config/sops/age/keys.txt` in `~/.bashrc` so the `sops` CLI picks it up automatically.

## One-time cluster bootstrap

Before any ArgoCD Application with SOPS-encrypted content can sync, the age **private** key must exist as a Kubernetes Secret in the `argocd` namespace so the KSOPS sidecar can decrypt:

```bash
kubectl create secret generic sops-age \
  --from-file=keys.txt="$HOME/.config/sops/age/keys.txt" \
  --namespace argocd

kubectl -n argocd annotate secret sops-age \
  argocd.argoproj.io/sync-options=Prune=false
```

This is the **only** secret created by hand. Every other sensitive value in this repo is sealed via SOPS and committed as a `.enc.yaml` file.

## Initial setup sequence (first-time, in order)

1. Generate age key pair locally, export `SOPS_AGE_KEY_FILE`, back up private key.
2. Commit `.sops.yaml` + this file (this commit).
3. Run the one-time cluster bootstrap above.
4. Install KSOPS as an argocd-repo-server sidecar (values patch, separate commit).
5. Smoke test with a throwaway encrypted ConfigMap.
6. Start migrating real secrets (external-dns TSIG, homepage widget creds, etc.).

Steps 4–6 happen in later commits; this document is the reference they'll all point back to.

## Daily workflow

**Create a new encrypted file:**
```bash
sops new-secret.enc.yaml
# opens $EDITOR with a blank YAML; on save, sops encrypts data/stringData values
```

**Edit in place:**
```bash
sops existing.enc.yaml
# decrypts, opens $EDITOR, re-encrypts on save
```

**View decrypted content without editing:**
```bash
sops -d existing.enc.yaml
```

**Re-encrypt existing files after `.sops.yaml` changes** (e.g. rotating keys, adding recipients):
```bash
sops updatekeys existing.enc.yaml
```

## Recovery procedures

### Cluster rebuilt, private key still available
1. Install KSOPS sidecar via ArgoCD Helm values.
2. Re-run the one-time cluster bootstrap above.
3. Sync ArgoCD — all `.enc.yaml` content decrypts normally.

### Private key lost (no workstation file, no password-manager backup)
- **Every `.enc.yaml` in the repo is permanently unreadable. There is no recovery.**
- Generate a fresh age key pair, update `.sops.yaml` with the new public key, and re-create every encrypted file from scratch by pulling live values from the cluster (`kubectl get secret ... -o yaml`) and re-encrypting.
- Don't let this happen. Back up the private key to a password manager on day one.

## Conventions

- Sensitive values always live in a `kind: Secret` wrapper. Helm charts reference them by name (`existingSecret`, `secretName`, etc.), not by inline value.
- Encrypted files are named `*.enc.yaml` (matched by `.sops.yaml` path_regex).
- Plaintext helm values and ArgoCD Application manifests stay readable; only the Secret YAMLs are encrypted.

## Safety checks (planned — not yet implemented)

SOPS encryption is **not automatic** — `sops` is a CLI tool you have to invoke explicitly. A file named `foo.enc.yaml` is only actually encrypted if you ran `sops` on it. Two layers of safety to add before real secret migration begins:

### Layer 1 — Pre-commit hook (local, fast feedback)

Rejects any staged `*.enc.yaml` file that isn't SOPS-encrypted. Shared via git so every clone gets the same protection after one-time install.

`.pre-commit-config.yaml` at repo root:
```yaml
repos:
  - repo: local
    hooks:
      - id: sops-encrypted
        name: Verify *.enc.yaml files are SOPS-encrypted
        entry: bash -c 'for f in "$@"; do grep -q "^sops:" "$f" || { echo "ERROR: $f matches *.enc.yaml but is NOT encrypted. Run: sops -e -i $f"; exit 1; }; done' --
        language: system
        files: '\.enc\.yaml$'

  # Useful generic hooks to add alongside
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.6.0
    hooks:
      - id: check-yaml
        args: [--allow-multiple-documents]
      - id: end-of-file-fixer
      - id: trailing-whitespace
      - id: check-merge-conflict
```

Per-workstation install (one time):
```bash
pip install pre-commit         # or brew install pre-commit
pre-commit install             # writes .git/hooks/pre-commit
```

Test:
```bash
echo "apiVersion: v1" > bad.enc.yaml   # plaintext, not encrypted
git add bad.enc.yaml
git commit -m test                     # ← blocked with clear error
```

### Layer 2 — GitHub Actions (remote, catches what slipped through)

Even with pre-commit installed, someone can always `--no-verify` or commit from another machine. GH Actions on every push/PR provides the last-line defense and covers more than just SOPS safety.

`.github/workflows/security.yml`:
```yaml
name: security-scan
on:
  push:
    branches: [main]
  pull_request:

jobs:
  gitleaks:
    # Detects committed API keys, tokens, passwords. Also catches SOPS leaks
    # that slipped through pre-commit, and scans full git history.
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: gitleaks/gitleaks-action@v2
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

  trivy-config:
    # IaC misconfiguration: Terraform, k8s manifests, Dockerfiles, helm charts.
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: aquasecurity/trivy-action@master
        with:
          scan-type: config
          severity: CRITICAL,HIGH
          exit-code: 1
          ignore-unfixed: true

  kube-linter:
    # k8s manifest quality (missing resource limits, privileged containers, ...)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: stackrox/kube-linter-action@v1
        with:
          directory: .
          format: sarif
          output-file: kube-linter.sarif
```

**Also turn on in GitHub repo settings:**
- *Secret scanning* + *Secret scanning push protection* (free for public repos, paid for private)
- *Dependabot security updates* (catches vulnerable dependencies if this repo ever grows a package.json / requirements.txt)

### Why both layers

| Scenario | pre-commit catches | GH Actions catches |
|---|---|---|
| You forget to run `sops` before `git commit` | ✅ | ✅ (backup) |
| You commit with `--no-verify` | ❌ | ✅ |
| Teammate clones fresh and hasn't run `pre-commit install` | ❌ | ✅ |
| Historical leaked secret in git history | ❌ (local only) | ✅ (gitleaks scans history) |
| Terraform resource exposing a public IP by accident | ❌ | ✅ (Trivy) |
| Pod spec missing resource limits | ❌ | ✅ (kube-linter) |

Pre-commit is your fast local loop; GH Actions is the line you can't bypass.
