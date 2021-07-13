# Acceptance Tests

## Requirements
- kind
- node

## Running
1. Start a kind cluster, see [here](https://kind.sigs.k8s.io/docs/user/ingress/#ingress-nginx) for more details

```
cat <<EOF | kind create cluster --name kind --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
EOF
```

2. Install ingress-nginx

```
VERSION=$(curl https://raw.githubusercontent.com/kubernetes/ingress-nginx/master/stable.txt)
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/$VERSION/deploy/static/provider/kind/deploy.yaml
```

3. Install amathea

```
helm install amalthea helm-chart/amalthea
```

4. Run the tests

```
npm install
npx mocha test.js
```

5. Cleanup

```
kind delete cluster --name kind
```