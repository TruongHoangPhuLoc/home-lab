# upgrade system

# disable swap
swapoff -a

# disable across reboots in /etc/fstab (warning: just workaround solution, its not perfect)
sed -i 's^\/swap^\#\/swap^g' /etc/fstab

# or we can mask unit file of swap by having it linked to /dev/null
systemctl mask swap.img.swap 

# install required packages
apt install -y curl gnupg2 software-properties-common apt-transport-https ca-certificates

# add needed modules
cat <<EOF | sudo tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

# activate modules
modprobe overlay
modprobe br_netfilter

# sysctl params required by setup, params persist across reboots
cat <<EOF | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

# Apply sysctl params without reboot
sysctl --system

# check modules
lsmod | grep br_netfilter
lsmod | grep overlay

# check sysctl parameters
sysctl net.bridge.bridge-nf-call-iptables net.bridge.bridge-nf-call-ip6tables net.ipv4.ip_forward

# install container runtime
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmour -o /etc/apt/trusted.gpg.d/docker.gpg --yes
add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
apt update
apt install containerd.io -y

# generate default containerd configuration
containerd config default | sudo tee /etc/containerd/config.toml >/dev/null 2>&1
# modify cgroup driver to systemd
sed -i 's/SystemdCgroup \= false/SystemdCgroup \= true/g' /etc/containerd/config.toml

# restart container runtime to take effect of changes
systemctl restart containerd

# enable container runtime
systemctl enable containerd

# create keyrings folder to store k8s signed key in case it doesnt exist earlier
mkdir -p /etc/apt/keyrings && chmod 755 /etc/apt/keyrings 

# download k8s key
curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.28/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring. --yes

# add k8s repo
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list

# install k8s packages as well as holding them to prevent from automatically upgrading
apt-get update
apt-get install -y kubelet kubeadm kubectl
apt-mark hold kubelet kubeadm kubectl
