helm repo add percona https://percona.github.io/percona-helm-charts/

helm show values percona/pmm > values.yaml

helm upgrade --install pmm --namespace pmm-mysql-monitoring --create-namespace -f values.yaml percona/pmm