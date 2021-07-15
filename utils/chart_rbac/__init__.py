import base64
import json
import subprocess as sp
import sys
import yaml


# Fake release name which will be used for resource naming
# (note that we're not installing a helm release).
RELEASE_NAME = "amalthea-dev-local"
# Name of the user / context we're adding to the local kube config.
DEV_CONTEXT_NAME = "amalthea-dev-local"


def get_resource_kinds(include_crd):
    rbac_resource_kinds = ["ServiceAccount", "Role", "RoleBinding"]
    if include_crd:
        rbac_resource_kinds.append("CustomResourceDefinition")
    return rbac_resource_kinds


def strip_newlines(string_in):
    """Tiny helper to strip newlines from json strings in CRD."""
    return string_in.replace("\\n", "")


def get_chart_resources(amalthea_namespace, server_namespaces, resource_kinds):
    """
    Render the chart and filter the output to get crds,
    service accounts, roles and role bindings.
    """

    namespaces_string = "{" + ",".join(server_namespaces) + "}"
    resource_yamls = (
        sp.check_output(
            f'helm template helm-chart/amalthea -n {amalthea_namespace} \
                --set "scope.namespaces={namespaces_string}"',
            shell=True,
        )
        .decode("utf-8")
        .replace("RELEASE-NAME", RELEASE_NAME)
        .split("---")
    )
    resource_dicts = [yaml.safe_load(_) for _ in resource_yamls]
    resource_dicts = [_ for _ in resource_dicts if _ is not None]
    resource_dicts = [_ for _ in resource_dicts if "kind" in _]
    return [_ for _ in resource_dicts if _["kind"] in resource_kinds]


def configure_local_dev(amalthea_namespace, server_namespaces, include_crd=True):
    """
    Set up a dev environment by installing the CRD, installing role,
    role binding and service account and configuring your current kubectl
    context to use this service account instead.
    """

    config = yaml.safe_load(sp.check_output("kubectl config view", shell=True))
    current_context = config["current-context"]
    current_cluster = [_ for _ in config["contexts"] if _["name"] == current_context][
        0
    ]["context"]["cluster"]

    # Create the rbac resources in the cluster using the original context
    for resource in get_chart_resources(
        amalthea_namespace, server_namespaces, get_resource_kinds(include_crd)
    ):
        sys.stdout.write(
            sp.check_output(
                f"echo '{strip_newlines(json.dumps(resource))}' | kubectl \
                    apply  -n {amalthea_namespace} -f -",
                shell=True,
            ).decode()
        )

    # Get the token for the newly created service account
    sa = json.loads(
        sp.check_output(
            [f"kubectl get sa -n {amalthea_namespace} {RELEASE_NAME}-amalthea -o json"],
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
        f"kubectl config set-credentials {DEV_CONTEXT_NAME} --token {token}", shell=True
    )
    sp.check_output(
        f"kubectl config set-context {DEV_CONTEXT_NAME} \
            --user={DEV_CONTEXT_NAME} --cluster={current_cluster}",
        shell=True,
    )
    sp.check_output(
        f"kubectl config use-context {DEV_CONTEXT_NAME}",
        shell=True,
    )


def cleanup_local_dev(
    admin_context_name, amalthea_namespace, server_namespaces, include_crd=True
):
    """
    Reset the previous context as default and clean up.
    """

    sp.check_output(f"kubectl config use-context {admin_context_name}", shell=True)
    sp.check_output(f"kubectl config delete-context {DEV_CONTEXT_NAME}", shell=True)
    sp.check_output(f"kubectl config delete-user {DEV_CONTEXT_NAME}", shell=True)

    # Delete the rbac resources in the cluster using the original context
    for resource in get_chart_resources(
        amalthea_namespace, server_namespaces, get_resource_kinds(include_crd)
    ):
        sys.stdout.write(
            sp.check_output(
                f"echo '{strip_newlines(json.dumps(resource))}' | kubectl \
                    delete -n {amalthea_namespace} -f -",
                shell=True,
            ).decode()
        )
