import json
import yaml
import kopf
from kubernetes import dynamic
import kubernetes.client as k8s_client
from kubernetes.client.rest import ApiException
from datetime import datetime, timezone, timedelta

from k8s_resources import get_resources_specs, get_resource_configs
import config


@kopf.on.startup()
def configure(settings, **kwargs):
    """
    Configure the operator - see https://kopf.readthedocs.io/en/stable/configuration/
    for options.
    """
    if config.kopf_operator_settings:
        try:
            for key, val in config.kopf_operator_settings.items():
                getattr(settings, key).__dict__.update(val)
        except AttributeError(e):
            print(f"Problem when configuring the Operator: {e}")


def create_namespaced_resource(client, api, method, **kwargs):
    """
    Create a k8s resource given and api, the right method for creation
    and the specs of the resource.
    """
    api_class = getattr(client, api)()
    api_method = getattr(api_class, method)
    try:
        return api_method(**kwargs)
    except ApiException as e:
        print(f"Exception when calling {api}.{method}: {e}\n")


@kopf.on.create("renku.io/v1alpha1", "JupyterServer")
def create_fn(spec, meta, **kwargs):
    """
    Watch the creation of jupyter server objects and create all
    the necessary k8s resources which implement the jupyter server.
    """
    metadata = kwargs["body"]["metadata"]
    spec = kwargs["body"]["spec"]

    resources_specs = get_resources_specs(metadata, spec)
    resource_configs = get_resource_configs(
        oidc_enabled=spec["auth"]["oidc"]["enabled"], api_only=True
    )

    children = {"extraResources": []}
    kopf.label(
        resources_specs["statefulset"]["spec"]["template"],
        labels={"renku.io/jupyterserver": metadata["name"]},
    )
    if "labels" in meta:
        kopf.label(
            resources_specs["statefulset"]["spec"]["template"], labels=meta["labels"]
        )

    for resource_key, resource_spec in resources_specs.items():
        kopf.label(resource_spec, labels={"renku.io/jupyterserver": metadata["name"]})
        kopf.adopt(resource_spec)
        children[resource_key] = create_namespaced_resource(
            k8s_client,
            resource_configs[resource_key]["api"],
            resource_configs[resource_key]["creation_method"],
            namespace=metadata["namespace"],
            body=resource_spec,
        ).metadata.uid

    for extra_resource in spec["extraResources"]:
        kopf.adopt(extra_resource["resourceSpec"])
        kopf.label(resource_spec, labels={"renku.io/jupyterserver": metadata["name"]})
        children["extraResources"].append(
            create_namespaced_resource(
                k8s_client,
                extra_resource["api"],
                extra_resource["creationMethod"],
                namespace=metadata["namespace"],
                body=extra_resource["resourceSpec"],
            ).metadata.uid
        )

    return {"created_resources": children}


@kopf.on.delete("renku.io/v1alpha1", "JupyterServer")
def delete_fn(namespace, spec, body, **kwargs):
    """
    A deletion handler who's only job is to trigger deletion of pvc and pod
    and then fail until both and pvc are gone.
    """
    pod_alive = True
    pvc_alive = True

    try:
        k8s_client.CoreV1Api().delete_namespaced_persistent_volume_claim(
            name=body["children"]["PersistentVolumeClaim"]["name"],
            namespace=namespace,
        )
    except (ApiException, KeyError):
        pass

    try:
        k8s_client.AppsV1Api().delete_namespaced_stateful_set(
            name=body["children"]["StatefulSet"]["name"],
            namespace=namespace,
        )
    except (ApiException, KeyError):
        pass

    try:
        k8s_client.CoreV1Api().read_namespaced_pod(
            name=body["children"]["Pod"]["name"],
            namespace=namespace,
        )
    except (ApiException, KeyError):
        pod_alive = False

    try:
        k8s_client.CoreV1Api().read_namespaced_persistent_volume_claim(
            name=body["children"]["PersistentVolumeClaim"]["name"],
            namespace=namespace,
        )
    except (ApiException, KeyError):
        pvc_alive = False

    if pvc_alive or pod_alive:
        raise Exception("Waiting for pod and pvc destruction")


@kopf.on.event("statefulset", labels={"renku.io/jupyterserver": kopf.PRESENT})
@kopf.on.event("pod", labels={"renku.io/jupyterserver": kopf.PRESENT})
@kopf.on.event("persistentvolumeclaim", labels={"renku.io/jupyterserver": kopf.PRESENT})
@kopf.on.event("ingress", labels={"renku.io/jupyterserver": kopf.PRESENT})
@kopf.on.event("service", labels={"renku.io/jupyterserver": kopf.PRESENT})
def child_monitoring(meta, name, namespace, body, event, status, **kwargs):
    """
    Update the custom object with the child status.
    """
    # We should only receive CRUD events...
    try:
        op = {"MODIFIED": "replace", "ADDED": "add", "DELETED": "remove"}[event["type"]]
    except KeyError:
        return

    parent_name = meta["labels"]["renku.io/jupyterserver"]

    # TODO: This extra query should be done only once and avoided later
    dynamic_client = dynamic.DynamicClient(k8s_client.api_client.ApiClient())
    jupyter_server_api = dynamic_client.resources.get(
        api_version="v1alpha1", kind="JupyterServer"
    )

    # We use the dynamic client for patching since we need
    # content_type="application/json-patch+json"

    json_patch = [
        {
            "op": op,
            "path": f"/children/{body['kind']}",
        }
    ]
    if op in ["add", "replace"]:
        json_patch[0]["value"] = {
            "uid": body["metadata"]["uid"],
            "name": body["metadata"]["name"],
            "status": body["status"],
        }

    jupyter_server_api.patch(
        namespace=namespace,
        name=parent_name,
        body=json_patch,
        content_type="application/json-patch+json",
    )
    # TODO: catch the case where we're trying to update a deleted object for cleaner logs


# Note: This is a very experimental feature and it's implementation is likely
#       to evolve over time. Use with care.
if config.reschedule_on_node_failure:

    @kopf.timer(
        "renku.io/v1alpha1", "JupyterServer", initial_delay=60, interval=15, idle=15
    )
    def clean_pods_on_dead_nodes(spec, **kwargs):
        """
        Periodically check all jupyter server objects for the health of their host
        node. Kill pods on unreachable/dead nodes with the sledgehammer. This brings
        a risk multiple containers writing to the same volume should the pod still
        be running on the unreachable node.
        """
        pod_status = k8s_client.CoreV1Api().read_namespaced_pod_status(
            namespace=kwargs["body"]["metadata"]["namespace"],
            name=f"{kwargs['body']['metadata']['name']}-0",
        )
        ready_cond = [
            cond for cond in pod_status.status.conditions if cond.type == "Ready"
        ][0]

        # Would be nice if this came as a boolean already...
        if not (ready_cond.status == "False" and pod_status.status.phase == "Running"):
            print("all good...")
            return

        status_age = (
            datetime.now(ready_cond.last_transition_time.tzinfo)
            - ready_cond.last_transition_time
        )

        if status_age > timedelta(minutes=1):
            k8s_client.CoreV1Api().delete_namespaced_pod(
                namespace=kwargs["body"]["metadata"]["namespace"],
                name=f"{kwargs['body']['metadata']['name']}-0",
                grace_period_seconds=0,
                propagation_policy="Background",
            )
