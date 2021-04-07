import yaml
import kopf
import kubernetes.client
from kubernetes.client.rest import ApiException


from k8s_resources import *


def create_resource(api, method, **kwargs):
    try:
        return getattr(api, method)(**kwargs)
    except ApiException as e:
        print(f"Exception when calling {api.__class__.__name__}.{method}: {e}\n")


def get_statefulset_manifest(namespace, name, spec):

    auth = spec["auth"]
    image = spec["notebookImage"]

    notebook_container_manifest = get_notebooks_container(
        name, image, spec["host"], auth["token"]
    )

    if "resources" in spec:
        notebook_container_manifest["resources"] = spec["resources"]
    if "extraVolumeMounts" in spec:
        notebook_container_manifest["volumeMounts"] += spec["extraVolumeMounts"]

    stateful_set_manifest = get_stateful_set(namespace, name)
    stateful_set_manifest["spec"]["template"]["spec"]["containers"].append(
        notebook_container_manifest
    )
    if "extraInitContainers" in spec:
        stateful_set_manifest["spec"]["template"]["spec"]["initContainers"] += spec[
            "extraInitContainers"
        ]
    if "extraContainers" in spec:
        stateful_set_manifest["spec"]["template"]["spec"]["containers"] += spec[
            "extraContainers"
        ]
    if "extraVolumes" in spec:
        stateful_set_manifest["spec"]["template"]["spec"]["volumes"] += spec[
            "extraVolumes"
        ]
    if "extraImagePullSecrets" in spec:
        stateful_set_manifest["spec"]["template"]["spec"]["extraImagePullSecrets"] = [
            {"name": secret["metadata"]["name"]}
            for secret in spec["extraImagePullSecrets"]
        ]

    if auth["kind"] == "oidc":
        stateful_set_manifest["spec"]["template"]["spec"]["containers"].append(
            get_proxy_container()
        )
        stateful_set_manifest["spec"]["template"]["spec"]["containers"].append(
            get_authentication_container(auth["oidc"])
        )
        stateful_set_manifest["spec"]["template"]["spec"]["containers"].append(
            get_authorization_container(auth["oidc"]["userId"])
        )
    return stateful_set_manifest


@kopf.on.create("renku.io", "v1", "sessions")
def create_fn(spec, **kwargs):

    # Compile a list of resources
    resources = []

    # Create the Kubernetes API objects.
    # TODO: The APIs are currently defined here AND in the YAML manifests.
    core_api = kubernetes.client.CoreV1Api()
    apps_api = kubernetes.client.AppsV1Api()
    extensions_api = kubernetes.client.ExtensionsV1beta1Api()

    name = kwargs["body"]["metadata"]["name"]
    namespace = kwargs["body"]["metadata"]["namespace"]

    pvc_manifest = get_pvc(namespace, name, "temporary", "1Gi")
    kopf.adopt(pvc_manifest)
    pvc = create_resource(
        core_api,
        "create_namespaced_persistent_volume_claim",
        namespace=namespace,
        body=pvc_manifest,
    )
    resources.append(pvc)

    for secret_manifest in spec["extraImagePullSecrets"]:
        kopf.adopt(secret_manifest)
        secret = create_resource(
            core_api,
            "create_namespaced_secret",
            namespace=namespace,
            body=secret_manifest,
        )
        resources.append(secret)

    stateful_set_manifest = get_statefulset_manifest(namespace, name, spec)
    kopf.adopt(stateful_set_manifest)
    stateful_set = create_resource(
        apps_api,
        "create_namespaced_stateful_set",
        namespace=namespace,
        body=stateful_set_manifest,
    )
    resources.append(stateful_set)

    if spec["auth"]["kind"] == "oidc":
        cm_manifest = get_config_map(namespace, name, spec["host"])
        kopf.adopt(cm_manifest)
        cm = create_resource(
            core_api,
            "create_namespaced_config_map",
            namespace=namespace,
            body=cm_manifest,
        )
        resources.append(cm)

    ingress_manifest = get_ingress(namespace, name, spec["host"], spec["path"])
    kopf.adopt(ingress_manifest)
    ingress = create_resource(
        extensions_api,
        "create_namespaced_ingress",
        namespace=namespace,
        body=ingress_manifest,
    )
    resources.append(ingress)

    service_manifest = get_service(namespace, name, spec["auth"]["kind"])
    kopf.adopt(service_manifest)
    service = create_resource(
        core_api,
        "create_namespaced_service",
        namespace=namespace,
        body=service_manifest,
    )
    resources.append(service)

    return {"children": [resource.metadata.uid for resource in resources]}
