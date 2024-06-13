# Amalthea - A Kubernetes operator for Jupyter servers

This project defines a `JupyterServer` [custom resource](manifests/crd.yaml) for
Kubernetes and implements a Kubernetes operator which controls the lifecycle of
custom `JupyterServer` objects.

## Installation

The recommended way of installing Amalthea is through its **helm chart**:

```bash
helm repo add renku https://swissdatasciencecenter.github.io/helm-charts
helm install amalthea renku/amalthea
```

For people who prefer to use plain manifests in combination with tools like
`kustomize`, we provide the rendered templates in the
[manifests directory](manifests), together with a basic `kustomization.yaml`
file which can serve as a base for overlays. A basic install equivalent to a
helm install using the default values can be achieved through

```bash
kubectl apply -k github.com/SwissDataScienceCenter/amalthea/manifests/
```

## Example

Once Amalthea is installed in a cluster through the helm chart, deploying a
jupyter server for a user `Jane Doe` with email `jane.doe@example.com` is as
easy as applying the following YAML file to the cluster:

```yaml
apiVersion: amalthea.dev/v1alpha1
kind: JupyterServer
metadata:
  name: janes-spark-session
  namespace: datascience-workloads
spec:
  jupyterServer:
    image: jupyter/all-spark-notebook:latest
  routing:
    host: jane.datascience.example.com
    path: /spark-session
    tls:
      enabled: true
      secretName: example-com-wildcard-tls
  auth:
    oidc:
      enabled: true
      issuerUrl: https://auth.example.com
      clientId: jupyter-servers
      clientSecret:
        value: 5912adbd5f946edd4bd783aa168f21810a1ae6181311e3c35346bebe679b4482
      authorizedEmails:
        - jane.doe@example.com
    token: ""
```

For the full configuration options check out the
[CRD documentation](docs/crd.md) as well as the
[section on patching](#patching-a-jupyterserver).

## What's "inside" a JupyterServer resource

The `JupyterServer` custom resource defines a bundle of standard Kubernetes
resources that handle the following aspects of running a Jupyter server in a
Kubernetes cluster:

- Routing through the creation of an ingress object and a service to expose the
  Jupyter server
- Access control through integration with existing OpenID Connect (OIDC)
  providers
- Some failure recovery thanks to running the Jupyter server using a
  statefulSet controller and by backing it with a persistent volume
  (optional).

When launching a Jupyter server, the custom resource spec is used to render the
jinja templates defined [here](controller/templates). The rendered templates are
then applied to the cluster, resulting in the creation of the following K8s
resources:

- A statefulSet whose pod spec has two containers, tha actual Jupyter server and
  an [oauth2 proxy](https://github.com/oauth2-proxy/oauth2-proxy) which is
  running in front of the Jupyter server
- A PVC which will be mounted into the Jupyter server
- A configmap to hold some non-secret configuration
- A secret to hold some secret configuration
- A service to expose the pod defined in the statefulSet
- An ingress to make the Jupyter server outside reachable from outside the cluster

## Patching a JupyterServer

We intentionally keep the configuration options through the jinja templates
relatively limited to cover only what we believe to be the frequent use cases.
However, as part of the custom resource spec, one can pass a list of
[json](https://datatracker.ietf.org/doc/html/rfc6902) or
[json merge](https://datatracker.ietf.org/doc/html/rfc7386) patches, which will
be applied to the resource specifications _after_ the rendering of the Jinja
templates. Through patching, one has the complete freedom to add, remove or
change K8s resources which are created as part of the custom resource object.

## Motivation and use cases

The main use case of Amalthea is to provide a layer on top of which developers
can build kubernetes-native applications that allow their users to spin-up and
manage Jupyter servers. We do not see Amalthea as a standalone tool
used by end users, as creating Jupyter servers with Amalthea requires access to
the Kubernetes API.

### Comparison to JupyterHub

[JupyterHub](https://jupyterhub.readthedocs.io/en/stable/) is the standard
application for serving Jupyter servers to multiple users. Unlike Amalthea,
JupyterHub _is_ designed to be an application for the end user to interact with,
and it can run on Kubernetes as well as on standalone servers. It therefore
comes "batteries included" with a web frontend, user management, a database that
keeps track of running servers, a configurable web proxy, etc.

The intended scope of Amalthea is much smaller than that. Specifically:

- Amalthea requires that there is already an OpenID Connect provider in the
  application stack.
- Amalthea itself is stateless. All state is stored as Kubernetes objects in
  etcd.
- Amalthea uses the Kubernetes-native ingress- and service concepts for
  dynamically adding and removing routes as Jupyter servers come and go, instead
  of relying on an additoinal proxy for routing.

## What's in the repo

The [helm-chart/amalthea](helm-chart/amalthea) directory contains a chart which
installs the custom resource definiton (optional) and the controller. The helm
chart templates therefore contain the
[Custom Resource Definition](helm-chart/amalthea/templates/crd.yaml) of the
`JupyterServer` resource. The [controller](controller) directory contains the
logic of that operator which is based on the very nice
[kopf framework](https://github.com/nolar/kopf).

## Testing Amalthea

The easiest way to try amalthea out is to install it in a K8s cluster. If you
don't have a K8s cluster handy you can also just use
[kind](https://kind.sigs.k8s.io/). Further sections in the documentation give
more details and information on how to do this.

After installing the helm chart you can start creating `jupyterserver`
resources.

Amalthea can work with any image from the
[Jupyter Docker Stacks](https://jupyter-docker-stacks.readthedocs.io/en/latest/using/selecting.html).
But you can also build your own using the Jupyter Docker Stacks Images as a base.
However, there are a few requirements for an image to work with Amalthea:

- The container should use port 8888.
- The configuration files at `/etc/jupyter/` should not be overwritten. But you
  have complete freedom to override these configurations by either (1) passing
  command line arguments to the `jupyter` command or start scripts or (2)
  creating configuration files in locations which are more preferred than
  `/etc/jupyter/` such as the `.jupyter` folder in the user home directory. See
  [here](https://jupyter.readthedocs.io/en/latest/use/jupyter-directories.html#configuration-files)
  for more information about which locations you can use to store and override
  the jupyter configuration.

## Amalthea development and contributing

You have found a bug or you are missing a feature? We would be happy to hear
from you, and even happier to receive a pull request :)

There are 2 ways to setup a development environment:

1. Using devcontainers
2. Using kind

Regardless of which option you chose you will need to have the following installed:
- poetry
- docker
- make

### Using devcontainers

If you are using VSCode, then you can simply open and start the devcontainer with VSCode.
If not read on.

1. Install the devcontainer CLI - https://github.com/devcontainers/cli
2. `devcontainer build --workspace-folder ./"
3. `devcontainer up --workspace-folder ./"
4. `devcontainer exec --workspace-folder ./ bash"
5. Run `make tests` inside the devcontainer 

Useful aliases for the devcontainer CLI:

```
alias dce="devcontainer exec --workspace-folder ./"
alias dcb="devcontainer build --workspace-folder ./"
alias dcu="devcontainer up --workspace-folder ./"
```

### Using kind

1. Install kind - https://kind.sigs.k8s.io/docs/user/quick-start#installation
2. `make kind_cluster`
3. Ensure that you switch your current k8s context to the kind cluster (this usually happens automatically)
4. `poetry install`
5. `make tests`

## Why is this project called Amalthea?

According to [Wikipedia](https://en.wikipedia.org/wiki/Amalthea), the name
Amalthea stands for:

- one of Jupiters many moons
- the foster-mother of Zeus (ie Jupiter)
- a unicorn
- a container ship

Also, it's another Greek name for something Kubernetes related.
