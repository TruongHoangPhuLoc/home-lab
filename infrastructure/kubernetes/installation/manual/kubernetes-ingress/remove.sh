kubectl delete -f config/crd/bases/k8s.nginx.org_virtualservers.yaml
kubectl delete -f config/crd/bases/k8s.nginx.org_virtualserverroutes.yaml
kubectl delete -f config/crd/bases/k8s.nginx.org_transportservers.yaml
kubectl delete -f config/crd/bases/k8s.nginx.org_policies.yaml
kubectl delete -f config/crd/bases/k8s.nginx.org_globalconfigurations.yaml

kubectl delete -f https://raw.githubusercontent.com/nginxinc/kubernetes-ingress/v3.5.0/deploy/crds.yaml