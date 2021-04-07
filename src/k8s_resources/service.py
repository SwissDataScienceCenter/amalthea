import yaml


def get_service(namespace, name, auth_kind):
    if auth_kind == "oidc":
        target_port = 8887
    else:
        target_port = 8888

    return yaml.safe_load(
        f"""
    kind: Service
    apiVersion: v1
    metadata:
      name: {name}
      namespace: {namespace}
      labels:
        app: {name}
    spec:
      ports:
        - name: http
          protocol: TCP
          port: 80
          targetPort: {target_port}
      selector:
        app: {name}
      ClusterIP: None
 """
    )
