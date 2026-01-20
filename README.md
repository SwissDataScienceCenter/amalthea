# Amalthea - A Kubernetes operator for user sessions

This project defines an AmaltheaSession which provides a research work
environment and implements a Kubernetes operator which controls the lifecycle of custom `AmaltheaSession` objects.


## Description

## Installation

The recommended way of installing Amalthea is through its **helm chart**:

```bash
helm repo add renku https://swissdatasciencecenter.github.io/helm-charts
helm install amalthea-sessions renku/amalthea
```

## Getting Started

### Prerequisites
- go version v1.20.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/amalthea:tag
```

**NOTE:** This image ought to be published in the personal registry you specified. 
And it is required to have access to pull the image from the working environment. 
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/amalthea:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin 
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Contributing
You have found a bug or you are missing a feature? We would be happy to hear
from you, and even happier to receive a pull request :)

There are 2 ways to setup a development environment:

1. Using devcontainers
2. Using kind

Regardless of which option you chose you will need to have the following installed:
- go
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
4. `make test`
5. `make test-e2d`

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## Developer documentation: remote sessions

This section documents how Amalthea supports remote sessions.

The Amalthea Session resource definition contains a field named `location` which is set to `local` for
local sessions and set to `remote` for remote sessions.
The default value for `location` is `local` which means that the user-defined container runs in the session's pod
alongside the extra containers for the session.

Setting `location` to `remote` allows users to run sessions on remote computing environments where the session
pod only runs sessions services.
The first use-case for remote sessions is to make use of HPC resources, but the remote session architecture
allows for remote sessions to be running on many types of computing environments.

### Remote session containers

When the `location` field is set to `remote`, there are some differences with `local` sessions:

1. The "main" container is now running the `remote-session-controller`, see: [sidecars](cmd/sidecars/main.go).
   
   This container is now responsible for starting the remote session. This is done by providing it with a suitable configuration in the `remoteSecretRef` which is loaded as environment variables.

2. A new "tunnel" container is added to establish network connections
   between the remote session and the Amalthea pod.

   The tunnel server accepts secured connections from the remote session
   so that network traffic for its frontend can be forwarded from the
   Amalthea session pod. This is a reverse proxy from the tunnel container to the remote session.

   The remote session also establishes a forward proxy to the git proxy
   so that it can be used by the remote session.

### Remote session ingress

The ingress for a `remote` session now has a new route, `__amalthea__/tunnel`, which exposes the tunnel service to the internet.

The tunnel service only accepts authorized connections to make sure that
only the remote session itself can make use of the tunnel service.

### Remote session controller

At the moment, the remote session controller can only start remote
sessions using the FirecREST API (deployed in HPC environments).
