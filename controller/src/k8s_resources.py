import base64
from deepmerge import always_merger
import os
import yaml
from urllib.parse import urljoin


TEMPLATE_DIR = os.path.join(os.path.dirname(__file__), "templates")

# TODO: The APIs are currently defined here AND in the YAML manifests.
# A List defining the api and api methods for creation for each resource.
# This could be inferred by looking at the manifest, would have to verify
# how this could work in the case of custom resources.
def get_resource_configs(auth_kind="token", api_only=False):
    minimal_resource_configs = {
        "jupyter-server": {"template": "jupyter-server.yaml"},
        "service": {
            "api": "CoreV1Api",
            "creation_method": "create_namespaced_service",
            "template": "service.yaml",
        },
        "pvc": {
            "api": "CoreV1Api",
            "creation_method": "create_namespaced_persistent_volume_claim",
            "template": "pvc.yaml",
        },
        "ingress": {
            "api": "ExtensionsV1beta1Api",
            "creation_method": "create_namespaced_ingress",
            "template": "ingress.yaml",
        },
        "statefulset": {
            "api": "AppsV1Api",
            "creation_method": "create_namespaced_stateful_set",
            "template": "statefulset.yaml",
        },
    }
    auth_resource_configs = {
        "configmap": {
            "api": "CoreV1Api",
            "creation_method": "create_namespaced_config_map",
            "template": "configmap.yaml",
        },
        # Those are containers which are part of the statefulset. We treat them
        # as individual resources to enable modification before we add those
        # container specs to the statefulset pod spec template.
        "auth-proxy": {"template": "auth-proxy.yaml"},
        "authorization-plugin": {"template": "authorization-plugin.yaml"},
        "authentication-plugin": {"template": "authentication-plugin.yaml"},
    }

    # TODO: This is ugly - make this nicer!
    if auth_kind == "oidc":
        resource_configs = {**minimal_resource_configs, **auth_resource_configs}
    else:
        resource_configs = minimal_resource_configs

    if api_only:
        return {
            key: config for (key, config) in resource_configs.items() if "api" in config
        }
    else:
        return resource_configs


def create_template_keys(metadata, spec):
    """Create a single non-nested dictionary which contains all the
    variables needed for the templating as keys."""
    return {
        # Metadata
        "name": metadata["name"],
        "namespace": metadata["namespace"],
        # Jupyter server
        "jupyter_image": spec["jupyterServer"]["image"],
        "jupyter_default_url": spec["jupyterServer"]["defaultUrl"],
        "jupyter_notebook_dir": spec["jupyterServer"]["notebookDir"],
        "jupyter_token": spec["auth"]["token"],
        # Routing
        "host": spec["routing"]["host"],
        "path": spec["routing"]["path"].rstrip("/"),
        "full_url": urljoin(
            f"https://{spec['routing']['host']}", spec["routing"]["path"].rstrip("/")
        ),
        "target_port": 8000 if spec["auth"]["kind"] == "oidc" else 8888,
        # Auth proxy and plugins
        "oidc_issuer_url": spec["auth"]["oidc"]["issuerUrl"],
        "oidc_client_id": spec["auth"]["oidc"]["clientId"],
        "oidc_client_secret": spec["auth"]["oidc"]["clientSecret"],
        "oidc_user_id": spec["auth"]["oidc"]["userId"],
        "authentication_cookie_secret": base64.urlsafe_b64encode(
            os.urandom(32)
        ).decode(),
        # Volume
        "volume_size": spec["volume"]["size"],
        "volume_storage_class": spec["volume"]["storageClass"],
    }


def render_template(template_file, template_keys):
    """
    Render the template given the keyword arguments and return
    a python dictionary specifying the resource.
    """

    tmpl_path = os.path.join(TEMPLATE_DIR, template_file)
    tmpl_string = open(tmpl_path, "rt").read()
    yaml_string = tmpl_string.format(**template_keys)
    resource_spec = yaml.safe_load(yaml_string)
    return resource_spec


def get_resource_dicts(metadata, spec):
    """
    Create the resource specifications (as nested python dictionaries) that
    make up the custom resource object. No input validation happens here, we
    rely on CRD schema validation.
    """

    template_keys = create_template_keys(metadata, spec)

    resource_configs = get_resource_configs(
        auth_kind=spec["auth"]["kind"], api_only=False
    )

    # Create a list of the resources to be created (as dictionaries).
    resource_dicts = {}
    for key, config in resource_configs.items():
        resource_dicts[key] = render_template(config["template"], template_keys)

    # Go through the list of modifications and apply them to the right resource.
    # Note that deepmerge modifies the resource_dict in-place
    for mod in spec["resourceModifications"]:
        always_merger.merge(resource_dicts[mod["resource"]], mod["modification"])

    container_keys = [
        key for key, config in resource_configs.items() if not "api" in config
    ]

    for container_key in container_keys:
        resource_dicts["statefulset"]["spec"]["template"]["spec"]["containers"].append(
            resource_dicts[container_key]
        )
        resource_dicts.pop(container_key, None)

    return resource_dicts
