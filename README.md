# Jupyter Server Operator

**This project is still in a very early stage. It might be dropped or take radical changes in direction.**

The goal of this project is to run jupyter servers in a kubernetes cluster as custom resources.

The custom resource object handles the following aspects of the jupyter server:
 - fault tolerance by using a statefulset to run the server and backing it with a PVC
 - exposing it through the creation of an ingress object and a service to expose the jupyter server
 - access control through a pre-defined token or through an existing OIDC provider

The `/helm-chart` directory contains a chart which installs the custom resource definiton and a controller watching the custom resource. The `controller` directory contains the logic of that controller which is based on the kopf framework. Finally, the `/authorization` directory contains a very simple service which can be used in combination with an off-the-shelf authentication plugin to protect the servers.

## Access control

There are two access control modes, `oidc` and `token`.

### Access control through a token

In this mode we simply define the token to be passed to the jupyter server as part of the custom resource and establish connection to the jupyter server. Whoever has the token can access the jupyter server.

### Access control through an OIDC provider

In this mode we run traefik as a reverse proxy inside the pod together with the jupyter server. This traefik proxy uses two [forward-auth middlewares](https://doc.traefik.io/traefik/middlewares/forwardauth/), one for [authentication](https://github.com/oauth2-proxy/oauth2-proxy) and one for [authorization](https://github.com/SwissDataScienceCenter/jupyter-server-operator/tree/main/authorization), which both run as seperate conatiners in the main pod too. The first plugin uses any configured OIDC provider to authenticates the incoming request. At the first request, this will trigger a redirection to the OIDC provider. The authentication has a session with the browser which holds the information about the authenticated user. The authentication plugin adds this information to the request headers before handing the request back to traefik. Traefik then forwards the request headers to the authorization pluging which checks that the user information matches some criteria which are specified in the spec of the custom resource (currently only a pre-defined user id) and thus authorizes (or denies) access. If access is authorized, traefik finally forwards the request to the jupyter server container which is running without any access control in this mode.
