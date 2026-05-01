# Bare-metal Docker container (not k8s) for the monitoring-server reverse
# proxy. Uses upstream nginx:latest which runs as root to bind ports 80/443
# directly on the host. TODO: switch to nginxinc/nginx-unprivileged with
# port mapping when the monitoring-server config is next revisited.
# trivy:ignore:AVD-DS-0002
FROM nginx:latest
COPY nginx.conf /etc/nginx/conf.d/monitoring-stack.central.internal.locthp.com.conf
