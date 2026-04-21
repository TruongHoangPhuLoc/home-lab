helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/

helm upgrade --install external-dns external-dns/external-dns --version 1.15.0 -f values.yaml \
    --create-namespace \
    --namespace external-dns