## Amalthea - A Kubernetes operator for Jupyter servers

This project defines a `JupyterServer` custom resoure for Kubernetes and implements a kubernetes operator which controls the lifecycle of custom JupyterServer objects.

**Warning: This project is still in a very early stage.**


The JupyterServer custom resource defines a bundle of standard kubernetes resources that handle the following aspects of running a jupyter server in a k8s cluster:
 - Routing through the creation of an ingress object and a service to expose the jupyter server
 - Access control through easy integration with an OIDC provider
 - Resilience against failures through a StatefulSet controller which runs the server and backing it with a PVC (optional).

## What's in the repo

The [helm-chart/amalthea](https://github.com/SwissDataScienceCenter/amalthea/tree/main/helm-chart/amalthea) directory contains a chart which installs the custom resource definiton (optional) and the controller. The [controller](https://github.com/SwissDataScienceCenter/amalthea/tree/main/controller) directory contains the logic of that operator which is based on the very nice [kopf framework](https://github.com/nolar/kopf). Finally, the [authorization](https://github.com/SwissDataScienceCenter/amalthea/tree/main/authorization) directory contains a very simple service which checks that id of the authenticated user.

## Access control using an OIDC provider

We run traefik as a reverse proxy inside the main pod together with the jupyter server. This traefik proxy uses two [forward-auth middlewares](https://doc.traefik.io/traefik/middlewares/forwardauth/), one for [authentication](https://github.com/oauth2-proxy/oauth2-proxy) and one for [authorization](https://github.com/SwissDataScienceCenter/jupyter-server-operator/tree/main/authorization), which both run as seperate conatiners in the main pod alongside the jupyter server too. The authentication plugin uses any configured OIDC provider to authenticate the incoming request. At the first request, this will trigger a redirection to the OIDC provider. The authentication pluging then creates a session with the browser that holds the information about the authenticated user. The authentication plugin adds this information to the request headers before handing the request back to traefik. Traefik then forwards the request headers to the authorization pluging checks that the authenticated user matches some criteria which are specified in the spec of the custom resource (currently only a pre-defined user id) and thus authorizes (or denies) access. If access is authorized, traefik finally forwards the request to the jupyter server container.

## Why Amalthea?

According to [Wikipedia](https://en.wikipedia.org/wiki/Amalthea), the name Amalthea stands for:
- one of Jupiters many moons
- the foster-mother of Zeus (ie Jupiter)
- a unicorn
- a container ship

Also, it's another Greek name for something Kubernetes related.

## Contributing

The easiest way to set up the environment that will let you develop and test a feature is to use [kind](https://kind.sigs.k8s.io/).
Kind runs a whole k8s cluster in docker and it can easily be used to run and test amalthea. We use kind for our integration
tests too. The integration tests will run in your current active k8s context in the `default` namespace. So it is
reccomended that you create a new kind cluster before you start the tests. Once you install kind, creating a cluster is as
easy as:

```bash
kind create cluster
```

This will create a cluster named kind and also set your k8s context to the new cluster.

The only other requirement is pipenv. In order to install it check the [instructions here](https://pipenv.pypa.io/en/latest/#install-pipenv-today)

To run the tests simply do:
```bash
pipenv run pytest
```
