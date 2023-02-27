# Acceptance Tests

## Requirements
- kind
- node

## Running
1. Start a `kind` cluster with the config below which will enable
`ingress-nginx` to be installed and used as an ingress controller.
See [here](https://kind.sigs.k8s.io/docs/user/ingress/#ingress-nginx) for more details
on installing `ingress-nginx` in kind.

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
VERSION=controller-v1.0.3
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/$VERSION/deploy/static/provider/kind/deploy.yaml
```

3. Install amathea

```
cd helm-chart
pipenv run chartpress
kind load docker-image $(pipenv run chartpress --list-images) 
helm install amalthea amalthea
```

4. Run the tests

```
npm install
npx mocha test.js
```

Alternatively to specify an image, environment (i.e. lab/rstudio) and a cypress test spec.
Set the following environment variables:

```
TEST_IMAGE_NAME=renku/renkulab-py:3.8-0.8.0 TEST_SPEC=jupyterlab.cy.js ENVIRONMENT=lab npx mocha test.js
```

5. Cleanup

```
kind delete cluster --name kind
```