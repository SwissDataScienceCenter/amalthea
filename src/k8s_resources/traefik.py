import yaml


def get_config_map(namespace, name, host):
    return yaml.safe_load(
        f"""
    kind: ConfigMap
    apiVersion: v1
    metadata:
      name: {name}
      namespace: {namespace}
      labels:
        app: session

    data:
      rules.toml: |
        [http]
          [http.routers]
            [http.routers.oauth]
              rule = "PathPrefix(`/oauth2/`)"
              middlewares = ["authHeaders"]
              service = "oauthProxy"

            [http.routers.addAuth]
              rule = "PathPrefix(`/`)"
              middlewares = ["oauth", "customAuthorization"]
              service = "notebook"


          [http.middlewares]


            [http.middlewares.customAuthorization.forwardauth]
              address = "http://localhost:3000"
              trustForwardHeader = true

            [http.middlewares.authHeaders.headers]
              browserXssFilter = true
              contentTypeNosniff = true
              forceSTSHeader = true
              sslHost = "{host}"
              stsIncludeSubdomains = true
              stsPreload = true
              frameDeny = true

            [http.middlewares.oauth.forwardAuth]
              address = "http://localhost:4180/"
              trustForwardHeader = true
              authResponseHeaders = ["X-Auth-Request-Access-Token", "Authorization", "X-Auth-Request-User", "X-Auth-Request-Groups", "X-Auth-Request-Email", "X-Auth-Request-Preferred-Username"]

          [http.services]
            [http.services.notebook.LoadBalancer]
              method = "drr"
              passHostHeader = false
              [[http.services.notebook.LoadBalancer.servers]]
                url = "http://localhost:8888/"
                weight = 1
            [http.services.oauthProxy.LoadBalancer]
              method = "drr"
              [[http.services.oauthProxy.LoadBalancer.servers]]
                url = "http://localhost:4180/"
                weight = 1

      traefik.toml: |

        [log]
          level = "debug"

        [api]
          dashboard = false

        [providers]
          [providers.file]
            directory = "/config"

        [entrypoints]
          [entrypoints.http]
            address = ":8887"

        [accessLog]
          bufferingSize = 1

  """
    )
