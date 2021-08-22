# Amalthea authorization plugin

This is a minimal implementation of an authorization plugin designed to work as
traefik `forwardAuth` middleware in combination with the oauth2 proxy. In its
current form it simply checks if the user ID added to the `x-auth-request-user`
field matches the user ID provided as environment variable. The development of
this could go in two directions:

- We get rid of this component completely and let the oauth2 proxy handle it.
  For this we'd have to contribute the functionality of checking user email
  or user ID to upstream.
- We see this as an entrypoint for enforcing rules which are configurable
  through the custom resource spec.
