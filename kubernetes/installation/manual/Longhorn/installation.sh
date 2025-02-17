kubectl create ns longhorn-system
kubectl apply -n longhorn-system -f https://raw.githubusercontent.com/longhorn/longhorn/v1.7.1/deploy/prerequisite/longhorn-iscsi-installation.yaml

kubectl apply -n longhorn-system -f https://raw.githubusercontent.com/longhorn/longhorn/v1.7.1/deploy/prerequisite/longhorn-nfs-installation.yaml

helm repo add longhorn https://charts.longhorn.io
helm repo update
helm upgrade --install longhorn longhorn/longhorn --namespace longhorn-system \
  -f values.yaml \
  --version v1.7.1

#Create ingress
USER=<USERNAME_HERE>; PASSWORD=<PASSWORD_HERE>; echo "${USER}:$(openssl passwd -stdin -apr1 <<< ${PASSWORD})" >> auth
kubectl -n longhorn-system create secret generic basic-auth --from-file=auth