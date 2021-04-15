import yaml
import kopf
import kubernetes.client as k8s_client
from kubernetes.client.rest import ApiException

from k8s_resources import get_resource_dicts, get_resource_configs


def create_namespaced_resource(client, api, method, **kwargs):
    """Create a k8s resource given and api, the right method for creation
    and the specs of the resource."""
    api_class = getattr(client, api)()
    api_method = getattr(api_class, method)
    try:
        return api_method(**kwargs)
    except ApiException as e:
        print(f"Exception when calling {api}.{method}: {e}\n")


@kopf.on.create("renku.io/v1alpha1", "JupyterServer")
def create_fn(spec, **kwargs):

    metadata = kwargs["body"]["metadata"]
    spec = kwargs["body"]["spec"]

    resource_dicts = get_resource_dicts(metadata, spec)
    resource_configs = get_resource_configs(
        auth_kind=spec["auth"]["kind"], api_only=True
    )

    resources = []
    for resource_key, resource_dict in resource_dicts.items():
        kopf.adopt(resource_dict)
        resources.append(
            create_namespaced_resource(
                k8s_client,
                resource_configs[resource_key]["api"],
                resource_configs[resource_key]["creation_method"],
                namespace=metadata["namespace"],
                body=resource_dict,
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
