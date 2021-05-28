import yaml
import kopf
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
def create_fn(spec, **kwargs):
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

    resources = []
    for resource_key, resource_spec in resources_specs.items():
        kopf.adopt(resource_spec)
        resources.append(
            create_namespaced_resource(
                k8s_client,
                resource_configs[resource_key]["api"],
                resource_configs[resource_key]["creation_method"],
                namespace=metadata["namespace"],
                body=resource_spec,
            )
        )

    for extra_resource in spec["extraResources"]:
        kopf.adopt(extra_resource["resourceSpec"])
        resources.append(
            create_namespaced_resource(
                k8s_client,
                extra_resource["api"],
                extra_resource["creationMethod"],
                namespace=metadata["namespace"],
                body=extra_resource["resourceSpec"],
            )
        )

    return {"children": [resource.metadata.uid for resource in resources]}


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
