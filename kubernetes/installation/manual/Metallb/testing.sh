kubectl create deployment nginx --image=nginx

kubectl expose deployment nginx --type=LoadBalancer --port=80