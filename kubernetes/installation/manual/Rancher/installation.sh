helm repo add rancher-latest https://releases.rancher.com/server-charts/latest
kubectl create namespace cattle-system


helm install rancher rancher-alpha/rancher \
  --namespace cattle-system \
  --set hostname=rancher.locthp.com \
  --set bootstrapPassword=admin \
  --set version="2.7.9"