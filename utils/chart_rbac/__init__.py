import base64
import json
import subprocess as sp
import yaml

from kubernetes import client, config, utils
from kubernetes.dynamic import DynamicClient, exceptions as k8s_dynamic_exc


# Fake release name which will be used for resource naming
# (note that we're not installing a helm release).
RELEASE_NAME = "amalthea-dev-local"
# Name of the user / context we're adding to the local kube config.
DEV_CONTEXT_NAME = "amalthea-dev-local"


def get_chart_resources(
    amalthea_namespace,
    server_namespaces,
    resource_kinds,
    release_name=RELEASE_NAME,
):
    """
    Render the chart and filter the output to get crds,
    service accounts, roles and role bindings.
    """

    namespaces_string = "{" + ",".join(server_namespaces) + "}"
    resource_yamls = (
        sp.check_output(
            f'helm template {release_name} helm-chart/amalthea -n {amalthea_namespace} \
                --set "scope.namespaces={namespaces_string}"',
            shell=True,
        )
        .decode("utf-8")
        .split("---")
    )
    resource_dicts = [yaml.safe_load(_) for _ in resource_yamls]
    resource_dicts = [_ for _ in resource_dicts if _ is not None]
    resource_dicts = [_ for _ in resource_dicts if "kind" in _]
    return [_ for _ in resource_dicts if _["kind"] in resource_kinds]


def create_k8s_resources(
    amalthea_namespace,
    server_namespaces,
    resources=["ServiceAccount", "Role", "RoleBinding", "CustomResourceDefinition"],
    release_name=RELEASE_NAME,
):
    """Create k8s resources from the amalthea helm chart."""
    config.load_kube_config()
    k8s_client = client.ApiClient()

    # Create the rbac resources in the cluster using the original context
    for resource in get_chart_resources(
        amalthea_namespace, server_namespaces, resources, release_name
    ):
        try:
            utils.create_from_dict(k8s_client, resource, namespace=amalthea_namespace)
        except utils.FailToCreateError as err:
            if all([exc.reason == "Conflict" for exc in err.api_exceptions]):
                # NOTE: All required resources already exist, do not error out
                pass
            else:
                raise


def cleanup_k8s_resources(
    amalthea_namespace,
    server_namespaces,
    resources=["ServiceAccount", "Role", "RoleBinding", "CustomResourceDefinition"],
    release_name=RELEASE_NAME,
):
    """
    Remove k8s resources created to set amalthea up.
    """
    config.load_kube_config()
    k8s_client = client.ApiClient()
    dc = DynamicClient(k8s_client)

    # Delete the rbac resources in the cluster using the original context
    for resource in get_chart_resources(
        amalthea_namespace,
        server_namespaces,
        resources,
        release_name,
    ):
        print(
            f"Trying to cleanup {resource['kind']} with name {resource['metadata']['name']}"
        )
        res_api = dc.resources.get(
            api_version=resource["apiVersion"], kind=resource["kind"]
        )
        try:
            res_api.delete(
                resource["metadata"]["name"],
                namespace=amalthea_namespace,
                propagation_policy="Foreground",
                async_req=False,
                grace_period_seconds=60,
                timeout=120,
                wait=True,
            )
        except k8s_dynamic_exc.NotFoundError:
            print(
                f"Could not find {resource['kind']} with name {resource['metadata']['name']}"
                ", skipping."
            )
        else:
            print(
                f"Succesfully cleaned up {resource['kind']} "
                f"with name {resource['metadata']['name']}"
            )


def configure_local_shell(
    amalthea_namespace,
    release_name=RELEASE_NAME,
    dev_context_name=DEV_CONTEXT_NAME,
):
    """
    Set the current k8s context in the shell to use amalthea's service account.
    """
    config = yaml.safe_load(sp.check_output("kubectl config view", shell=True))
    current_context = config["current-context"]
    current_cluster = [_ for _ in config["contexts"] if _["name"] == current_context][
        0
    ]["context"]["cluster"]

    # Get the token for the newly created service account
    sa = json.loads(
        sp.check_output(
            [f"kubectl get sa -n {amalthea_namespace} {release_name} -o json"],
            shell=True,
        )
    )
    sa_token_secret = yaml.safe_load(
        sp.check_output(
            [
                f"kubectl get secret -n {amalthea_namespace} {sa['secrets'][0]['name']} -o yaml"
            ],
            shell=True,
        )
    )
    token = base64.b64decode(sa_token_secret["data"]["token"].encode()).decode()

    # Use new context
    sp.check_output(
        f"kubectl config set-credentials {dev_context_name} --token {token}", shell=True
    )
    sp.check_output(
        f"kubectl config set-context {dev_context_name} \
            --user={dev_context_name} --cluster={current_cluster}",
        shell=True,
    )
    sp.check_output(
        f"kubectl config use-context {dev_context_name}",
        shell=True,
    )


def cleanup_local_shell(admin_context_name, dev_context_name=DEV_CONTEXT_NAME):
    """
    Reset the previous context as default and clean up.
    """
    sp.check_output(f"kubectl config use-context {admin_context_name}", shell=True)
    sp.check_output(f"kubectl config delete-context {dev_context_name}", shell=True)
    sp.check_output(f"kubectl config delete-user {dev_context_name}", shell=True)
