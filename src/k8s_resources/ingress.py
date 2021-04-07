import yaml


def get_ingress(namespace, name, host, path):
    return yaml.safe_load(
        f"""
        kind: Ingress
        apiVersion: extensions/v1beta1
        metadata:
          name: {name}
          namespace: {namespace}
          labels:
            app: {name}
          annotations:
            cert-manager.io/cluster-issuer: letsencrypt-production
            kubernetes.io/ingress.class: nginx
            kubernetes.io/tls-acme: "true"
            nginx.ingress.kubernetes.io/proxy-body-size: "0"
            nginx.ingress.kubernetes.io/proxy-buffer-size: 8k
            nginx.ingress.kubernetes.io/proxy-request-buffering: "off"
        spec:
          tls:
            - hosts:
                - {host}
              secretName: {name}-session-tls
          rules:
            - host: {host}
              http:
                paths:
                - path: {path}
                  backend:
                    serviceName: {name}
                    servicePort: 80
        """
    )
