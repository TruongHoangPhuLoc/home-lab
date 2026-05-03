# snapshot-controller (kubernetes-csi/external-snapshotter)

Cluster-wide CSI VolumeSnapshot controller + CRDs. Required because
rook-ceph's CSI provisioner sidecars include the upstream
`csi-snapshotter` which watches `VolumeSnapshot` / `VolumeSnapshotContent`
resources, but those CRDs are not bundled with rook-ceph — the
[`kubernetes-csi/external-snapshotter`](https://github.com/kubernetes-csi/external-snapshotter)
project owns them and expects to be installed once per cluster,
out-of-band.

## Why not ArgoCD-managed

Tried it as an ArgoCD Application wrapping the Piraeus chart and rolled
it back: too much complexity for fire-and-forget upstream manifests
that we don't customize. A two-line `kubectl apply -k` against the
upstream kustomize bases gets us the same outcome and stays out of the
GitOps tracking surface.

## Install / upgrade

Pin to a specific release tag (do not track `master`). Upgrade by
re-running with a new `?ref=` and approving any CRD diffs.

```bash
REF=v8.5.0   # external-snapshotter release tag

# CRDs (cluster-scoped)
kubectl --context home-cluster apply -k \
  "github.com/kubernetes-csi/external-snapshotter/client/config/crd?ref=${REF}"

# snapshot-controller Deployment + RBAC (kube-system)
kubectl --context home-cluster apply -k \
  "github.com/kubernetes-csi/external-snapshotter/deploy/kubernetes/snapshot-controller?ref=${REF}"
```

## Verify

```bash
kubectl --context home-cluster get crd | grep snapshot.storage.k8s.io
# expect 6 lines (3 volumesnapshot* + 3 volumegroupsnapshot*)

kubectl --context home-cluster -n kube-system get pod -l app.kubernetes.io/name=snapshot-controller
# expect 2 snapshot-controller-* pods Running 1/1 (HA pair, leader-elected)
```

## Uninstall

```bash
kubectl --context home-cluster delete -k \
  "github.com/kubernetes-csi/external-snapshotter/deploy/kubernetes/snapshot-controller?ref=${REF}"
kubectl --context home-cluster delete -k \
  "github.com/kubernetes-csi/external-snapshotter/client/config/crd?ref=${REF}"
```

Beware: deleting the CRDs deletes any `VolumeSnapshot` / `VolumeSnapshotContent`
resources that exist on the cluster, which is destructive if any backup
relies on them.

## Current install on this cluster

Installed via the upstream kustomize bases at `ref=v8.5.0` on 2026-05-02
after a brief detour through the Piraeus chart (rolled back as overkill).
Reproduced by running the two `kubectl apply -k` commands above.
