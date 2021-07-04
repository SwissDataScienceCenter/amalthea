from kubernetes.dynamic.exceptions import NotFoundError


def find_resource(name, namespace, k8s_api):
    try:
        res = k8s_api.get(name, namespace=namespace)
    except NotFoundError:
        return None
    else:
        return res.to_dict()


def is_pod_ready(pod):
    if pod is None:
        return False
    container_statuses = pod["status"]["containerStatuses"]
    return (
        container_statuses is not None
        and len(container_statuses) > 0
        and all([cs["ready"] for cs in container_statuses])
        and pod["metadata"].get("deletionTimestamp") is None
    )
