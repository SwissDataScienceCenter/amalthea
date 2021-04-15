# Jupyter Server Operator

**This project is still in a very early stage. It might be dropped or take radical changes in direction.**

The goal of this project is to run jupyter servers in a kubernetes cluster as custom resources.

The custom resource object handles the following aspects of the jupyter server:
 - fault tolerance by using a statefulset to run the server and backing it with a PVC
 - exposing it through the creation of an ingress object and a service to expose the jupyter server
 - authentication with an OIDC provider and authorization (both optional).

The `/helm-chart` directory contains a chart which installs the custom resource definiton and a controller watching the custom resource. The `controller` directory contains the logic of that controller which is based on the kopf framework. Finally, the `/authorization` directory contains a very simple service which can be used in combination with an off-the-shelf authentication plugin to protect the servers.

## Access control

There are two access control modes, "oidc" and "token".

### Access control through a token

In this mode we simply define the token to be passed to the jupyter server as part of the custom resource and establish connection to the jupyter server. Whoever has the token can access the jupyter server.

### Access control through an OIDC provider

In this mode we run traefik as a proxy inside the same pod together with the jupyter server. This traefik proxy uses two `auth-middlewares`, one for authentication and one for authorization, which both run as seperate conatiners
in the same pod too. The first plugin authenticates the incoming request by using a configured OIDC connect provider. I adds information about the user to the request headers before handing the request back to traefik. Traefik then forwards the request headers to the authorization pluging whic checks io the user
information matches some criteria (currently only a pre-defined user id) and authorizes (or denies) access. If access is authorized, traefik finally forwards the request to the jupyter server container which is running without any access control in this mode.
