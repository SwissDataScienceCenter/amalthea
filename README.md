# Amalthea - A Kubernetes operator for Jupyter servers

This project defines a `JupyterServer` custom resoure (CRD) for Kubernetes and implements a Kubernetes operator which controls the lifecycle of custom JupyterServer objects.

**Warning: This project is still in an early stage.**

The JupyterServer custom resource defines a bundle of standard Kubernetes resources that handle the following aspects of running a Jupyter server in a k8s cluster:

- Routing through the creation of an ingress object and a service to expose the Jupyter server
- Access control through easy integration with an OpenID Connect (OIDC) provider
- Resilience against failures through a StatefulSet controller which runs the server and optionally backing it with a persistent volume claim (PVC).

## What's in the repo

The [helm-chart/amalthea](https://github.com/SwissDataScienceCenter/amalthea/tree/main/helm-chart/amalthea) directory contains a chart which installs the custom resource definiton (optional) and the controller. The [controller](https://github.com/SwissDataScienceCenter/amalthea/tree/main/controller) directory contains the logic of that operator which is based on the very nice [kopf framework](https://github.com/nolar/kopf). Finally, the [authorization](https://github.com/SwissDataScienceCenter/amalthea/tree/main/authorization) directory contains a very simple service which checks that id of the authenticated user.

## Using Amalthea
The easiest way to try amalthea out is to install it in a k8s cluster. If you dont have a k8s cluster handy you can also just use [kind](https://kind.sigs.k8s.io/). Further sesctions in the documentation give more details and information on how to do this.

After installing the helm chart you can start creating `jupyterserver` resources. We have example manifests that are ready to be deployed with just a few edits in the `examples` folder in this repository. 

Amalthea can work with any image from the [Jupyter Docker Stacks](https://jupyter-docker-stacks.readthedocs.io/en/latest/using/selecting.html). But you can also build your own using the Juyter Docker Stacks Images as a base. However, there are a few requirements for an image to work with Amalthea:
- The container should should use port 8888.
- The configuration files at `/etc/jupyter/` should not be overwritten. But you have complete freedom to override these configurations by either (1) passing command line arguments to the `jupyter` command or start scripts or (2) creating configuration files in locations which are more preferred than `/etc/jupyter/` such as the `.jupyter` folder in the user home directory. See [here](https://jupyter.readthedocs.io/en/latest/use/jupyter-directories.html#configuration-files) for more information about which locations you can use to store and override the jupyter configuration.

## Access control using an OIDC provider

We run traefik as a reverse proxy inside the main pod together with the Jupyter server. This traefik proxy uses two [forward-auth middlewares](https://doc.traefik.io/traefik/middlewares/forwardauth/), one for [authentication](https://github.com/oauth2-proxy/oauth2-proxy) and one for [authorization](https://github.com/SwissDataScienceCenter/jupyter-server-operator/tree/main/authorization), which both run as seperate conatiners in the main pod alongside the Jupyter server too. The authentication plugin uses any configured OIDC provider to authenticate the incoming request. At the first request, this will trigger a redirection to the OIDC provider. The authentication plugin then creates a session with the browser that holds the information about the authenticated user. The authentication plugin adds this information to the request headers before handing the request back to traefik. Traefik then forwards the request headers to the authorization plugin checks that the authenticated user matches some criteria which are specified in the spec of the custom resource (currently only a pre-defined user id) and thus authorizes (or denies) access. If access is authorized, traefik finally forwards the request to the Jupyter server container.

## Amalthea development and contributing

You have found a bug or you are missing a feature? We would be happy to hear from you, and even happier to receive a
pull request :)

### Requirements

For Amalthea development you will need python 3, [pipenv](https://pipenv.pypa.io/en/latest/#install-pipenv-today),
[kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation), [kubectl](https://Kubernetes.io/docs/tasks/tools/#kubectl) and [helm](https://helm.sh/docs/intro/install/).

### Kind

The easiest way to set up a cluster that will let you develop and test a feature is to use
[kind](https://kind.sigs.k8s.io/). Kind runs a whole k8s cluster in docker and it can easily
be used to run and test amalthea. We use kind for our integration tests too.

### Running Kopf/Amalthea locally

During development, Kopf based operators can be executed locally using your local kubectl context. See
`kopf run --help` for more information. In oder to do this, you first need to install the `JupyterServer`
custom resource definiton defined in the helm charts template directory. It is also convenient to develop
Amalthea using a kubectl context which has the same (minimal) roles assigned that Amalthea will run with
when deployed through the helm chart. For this purpose, we provide a small script `utils/configure_local_dev.py`
which creates a service account together with a role and a role binding and configures a kubectl context
that uses this service account.

### Example for a development workflow based on kind

After cloning the repository and installing the required dependencies, executing the following commands
should get you up and running:

```bash
pipenv install --dev
kind create cluster
kubectl create ns amalthea-testing
pipenv run utils/configure_local_dev.py -n amalthea-testing
pipenv run kopf run --dev -n amalthea-testing kopf_entrypoint.py
```

Unfortunately, [kopf auto reloading](https://github.com/nolar/kopf/issues/237) is not yet implemented.
Therefore, after editing the code, you have to terminate and restart kopf. Once you are done working
and you want to remove any traces of Amalthea from your cluster and your kubectl context, run

```bash
pipenv run utils/cleanup_local_dev.py -n amalthea-testing --use-context kind-kind
```

Note that `kind-kind` should be replaced with the name of the context that you would like to set as default
after removing the context which has been created during the test execution. Finally, if you also want to
remove your kind cluster, run

```bash
kind delete cluster
```

### Testing

A combination of unit- and integration tests are executed through pytest. The integration tests run in the
`default` namespace of the cluster defined in your current kubectl context, and they will temporarily modify
your kubectl config to use a dedicated context with mimimal access right for the test execution. Furthermore,
the tests will temporarily install the JupyterServer custom resource definition (CRD), so if you already have
that CRD installed, please delete it before running the tests. By installing the CRD in the \tests we ensure
that the correct, up-to-date CRD is being tested and not an older version left over from past work or tests.
Overall we thus recommend that you create a new kind cluster to run the tests.

In a fresh cluster you can run the test suite by executing

```bash
pipenv run pytest
```

in the root directory of the repository.

## Why is this project called Amalthea?

According to [Wikipedia](https://en.wikipedia.org/wiki/Amalthea), the name Amalthea stands for:

- one of Jupiters many moons
- the foster-mother of Zeus (ie Jupiter)
- a unicorn
- a container ship

Also, it's another Greek name for something Kubernetes related.

