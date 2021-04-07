import base64
import os
import yaml


def get_notebooks_container(name, image, host, token):
    return yaml.safe_load(
        f"""
    name: notebook
    image: {image}
    command:
      - "jupyter"
      - "lab"
    args:
      - "--ServerApp.ip=0.0.0.0"
      - "--ServerApp.port=8888"
      - "--ServerApp.token={token}"
      - "--ServerApp.allow_origin=https://{host}"
    volumeMounts:
      - name: workspace
        mountPath: "/home/jovyan/scratch"
        subPath: scratch
    ports:
      - name: notebook-port
        containerPort: 8888
        protocol: TCP
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    imagePullPolicy: Always
    securityContext:
      runAsUser: 1000
      runAsGroup: 100
  """
    )


def get_proxy_container():
    return yaml.safe_load(
        f"""
    name: session-proxy
    image: "traefik:2.1.4"
    args:
      - "--configfile=/config/traefik.toml"
    ports:
      - name: http
        containerPort: 8887
        protocol: TCP
    resources:
      requests:
        cpu: 20m
        memory: 16Mi
      limits:
        cpu: 100m
        memory: 32Mi
    volumeMounts:
      - name: traefik-configmap
        mountPath: /config
    livenessProbe:
      tcpSocket:
        port: 8887
      initialDelaySeconds: 10
      timeoutSeconds: 2
      periodSeconds: 10
      successThreshold: 1
      failureThreshold: 3
    readinessProbe:
      tcpSocket:
        port: 8887
      initialDelaySeconds: 10
      timeoutSeconds: 2
      periodSeconds: 10
      successThreshold: 1
      failureThreshold: 1
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    imagePullPolicy: IfNotPresent
  """
    )


def get_authentication_container(auth):
    return yaml.safe_load(
        f"""
    name: authentication-plugin
    image: "bitnami/oauth2-proxy:latest"
    args:
      - "--provider=oidc"
      - "--client-id={auth['clientId']}"
      - "--oidc-issuer-url={auth['issuerUrl']}"
      - "--client-secret={auth['clientSecret']}"
      - "--session-cookie-minimal"
      - "--set-xauthrequest"
      - "--cookie-secret={base64.urlsafe_b64encode(os.urandom(32)).decode()}"
      - "--email-domain=*"
      - "--http-address=:4180"
      - "--skip-provider-button"
      - "--insecure-oidc-allow-unverified-email"
      - "--reverse-proxy"
      - "--upstream=static://202"
    ports:
      - name: http
        containerPort: 4180
        protocol: TCP
    resources:
      requests:
        cpu: 20m
        memory: 16Mi
      limits:
        cpu: 100m
        memory: 32Mi
  """
    )


def get_authorization_container(user_id):
    return yaml.safe_load(
        f"""
    name: authorization-plugin
    image: ableuler/auth-test:0.0.0-1
    env:
      - name: USER_ID
        value: "{user_id}"
    ports:
      - name: http
        containerPort: 3000
        protocol: TCP
    resources:
      requests:
        cpu: 20m
        memory: 16Mi
      limits:
        cpu: 100m
        memory: 32Mi
  """
    )


def get_stateful_set(namespace, name):
    return yaml.safe_load(
        f"""
    kind: StatefulSet
    apiVersion: apps/v1
    metadata:
      namespace: {namespace}
      name: {name}
    spec:
      selector:
        matchLabels:
          app: {name}
      serviceName: {name}
      replicas: 1 # Don't change this
      template:
        metadata:
          labels:
            app: {name}
        spec:
          initContainers: []
          containers: []
          volumes:
            - name: traefik-configmap
              configMap:
                name: {name}
                defaultMode: 420
            - name: workspace
              persistentVolumeClaim:
                claimName: {name}
          terminationGracePeriodSeconds: 30
          automountServiceAccountToken: false
          securityContext:
            fsGroup: 100
    """
    )
