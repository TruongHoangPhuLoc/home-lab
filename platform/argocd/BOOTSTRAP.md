# ArgoCD + SOPS bootstrap

Secret management in this repo uses [SOPS](https://github.com/getsops/sops) with [age](https://github.com/FiloSottile/age) encryption. Encrypted files live in git as `*.enc.yaml`; a KSOPS plugin runs as a sidecar in `argocd-repo-server` and decrypts them at render time before ArgoCD applies manifests to the cluster.

Configuration lives in [`/.sops.yaml`](../../.sops.yaml) at the repo root.

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
