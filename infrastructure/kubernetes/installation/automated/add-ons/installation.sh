helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm pull prometheus-community/kube-prometheus-stack --untar

helm upgrade kube-prometheus-stack \
    --create-namespace \
    --namespace monitoring ./ \
    -f ./values.yaml

helm uninstall kube-prometheus-stack --namespace monitoring
