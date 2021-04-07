import yaml


def get_pvc(namespace, name, storage_class, size):
    return yaml.safe_load(
        f"""
      kind: PersistentVolumeClaim
      apiVersion: v1
      metadata:
        name: {name}
        namespace: {namespace}
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: {size}
        storageClassName: {storage_class}

 """
    )
