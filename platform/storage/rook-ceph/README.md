# Rook-Ceph — operations

Day-to-day commands. For *why* the values look the way they do, read [`ARCHITECTURE.md`](./ARCHITECTURE.md).

## Lifecycle

Both releases stay **helm-managed** (deliberate exception — see [`platform/CLAUDE.md`](../../CLAUDE.md) → "Exceptions"). Do not create an `application.yaml` here.

```bash
# Operator (rare to upgrade — pinned chart matches operator image)
helm --kube-context home-cluster upgrade --install rook-ceph rook-ceph \
  --repo https://charts.rook.io/release \
  --version v1.19.3 \
  -n rook-ceph \
  -f platform/storage/rook-ceph/operator/values.yaml

# Cluster (where you change things — pools, RGW, dashboard, etc.)
helm --kube-context home-cluster upgrade --install rook-ceph-cluster rook-ceph-cluster \
  --repo https://charts.rook.io/release \
  --version v1.19.3 \
  -n rook-ceph \
  -f platform/storage/rook-ceph/cluster/values.yaml
```

The chart name is `rook-ceph-cluster` (not `rook/rook-ceph-cluster`) because we pass `--repo` directly. Same convention as the ArgoCD release.

## Health check

```bash
# Quick status from outside the cluster
kubectl --context home-cluster -n rook-ceph exec deploy/rook-ceph-tools -- ceph status
kubectl --context home-cluster -n rook-ceph exec deploy/rook-ceph-tools -- ceph df

# Verbose health (when status shows HEALTH_WARN)
kubectl --context home-cluster -n rook-ceph exec deploy/rook-ceph-tools -- ceph health detail
```

Common warnings and what they mean:

| Warning | Meaning | Action |
|---|---|---|
| `HEALTH_WARN slow ops` on BlueStore | OSD waited > threshold for a write to commit. Almost always HDD latency under load. | Check load; ignore if intermittent. |
| `HEALTH_WARN mons clock skew` | One mon's clock drifted from the others. | Check NTP on workers. |
| `HEALTH_WARN PG_BACKFILL_FULL` | An OSD is too full for backfill to finish. | Add capacity or reweight OSDs. |
| `HEALTH_ERR PG_DEGRADED` | Replicas missing — actual data risk. | Diagnose immediately; do not delete OSDs/hosts until recovery completes. |

## Provisioning a bucket (S3)

Workloads request buckets via `ObjectBucketClaim`:

```yaml
apiVersion: objectbucket.io/v1alpha1
kind: ObjectBucketClaim
metadata:
  name: my-bucket
  namespace: my-app
spec:
  generateBucketName: my-bucket
  storageClassName: ceph-bucket
```

Once the OBC is `Bound`, the bucket provisioner will have created in the same namespace:

- **`Secret/my-bucket`** — keys `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`.
- **`ConfigMap/my-bucket`** — keys `BUCKET_HOST`, `BUCKET_PORT`, `BUCKET_NAME`, `BUCKET_REGION`.

Wire your workload to those — never hardcode the bucket name (it gets a random suffix). Example for an env-from pattern:

```yaml
envFrom:
  - secretRef:
      name: my-bucket
  - configMapRef:
      name: my-bucket
```

The S3 endpoint inside the cluster is `http://rook-ceph-rgw-ceph-objectstore.rook-ceph.svc:80` — that's also what `BUCKET_HOST`/`BUCKET_PORT` resolve to.

## Inspecting an existing bucket

```bash
# List buckets in the object store
kubectl --context home-cluster -n rook-ceph exec deploy/rook-ceph-tools -- \
  radosgw-admin bucket list

# Show stats for one bucket
kubectl --context home-cluster -n rook-ceph exec deploy/rook-ceph-tools -- \
  radosgw-admin bucket stats --bucket=<name>

# Use s3cmd / aws CLI from a pod with the OBC's secret mounted (or copy creds locally)
```

## External access (future)

When we want external clients (e.g., terraform with a Ceph S3 backend) to reach RGW, add an Ingress:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ceph-s3
  namespace: rook-ceph
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    external-dns.alpha.kubernetes.io/hostname: s3.prod-cluster.internal.locthp.com
spec:
  ingressClassName: cilium
  tls:
    - hosts: [s3.prod-cluster.internal.locthp.com]
      secretName: ceph-s3-tls
  rules:
    - host: s3.prod-cluster.internal.locthp.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: rook-ceph-rgw-ceph-objectstore
                port:
                  number: 80
```

TLS terminates at the Ingress (cert-manager issues), HTTP between Ingress and RGW. Same pattern as the rest of the platform.

## Dashboard

- URL: `https://ceph-dashboard.prod-cluster.internal.locthp.com`
- Default user: `admin`
- Initial password:
  ```bash
  kubectl --context home-cluster -n rook-ceph get secret rook-ceph-dashboard-password \
    -o jsonpath='{.data.password}' | base64 -d ; echo
  ```

## Troubleshooting

| Symptom | First check |
|---|---|
| New OBC stuck `Pending` | `kubectl logs deploy/rook-ceph-operator -n rook-ceph` for bucket-provisioner errors |
| Ceph status `HEALTH_WARN OSD slow ops` | `ceph osd perf` to find the slow OSD; usually HDD contention |
| Pool full warning | `ceph df` to see which pool; add capacity, don't delete data under pressure |
| RGW pod CrashLoopBackoff after upgrade | Pool creation may be in flight — check `ceph osd pool ls` for the new RGW pools |
| `kubectl get cephobjectstore` shows `Reconciling` forever | `kubectl describe cephobjectstore` and `kubectl logs deploy/rook-ceph-operator` |
