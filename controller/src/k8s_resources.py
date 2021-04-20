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
def get_resource_configs(oidc_enabled=False, api_only=False):
    """
    Get a dictionary with all resources that should be created. For each
    resource we return the api, the api method and the resource manifest
    as dictionary. An exception to this are containers which we add to
    the statefulset pod spec. We treat those as individual resources at
    this point to simplify their modification.
    """
    resource_configs = {
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
        "configmap": {
            "api": "CoreV1Api",
            "creation_method": "create_namespaced_config_map",
            "template": "configmap.yaml",
        },
        # These containers will be added to the statefulset.
        "jupyter-server": {"template": "jupyter-server.yaml"},
        "auth-proxy": {"template": "auth-proxy.yaml"},
    }

    if oidc_enabled:
        resource_configs.update(
            {
                # These containers will be added to the statefulset.
                "authorization-plugin": {"template": "authorization-plugin.yaml"},
                "authentication-plugin": {"template": "authentication-plugin.yaml"},
            }
        )

    if api_only:
        return {
            key: config for (key, config) in resource_configs.items() if "api" in config
        }
    else:
        return resource_configs


def create_template_values(metadata, spec):
    """
    Create a single non-nested dictionary which contains all the
    variables needed for the templating as keys.
    """
    template_values = {
        # Metadata
        "name": metadata["name"],
        # Jupyter server
        "jupyter_image": spec["jupyterServer"]["image"],
        "jupyter_default_url": spec["jupyterServer"]["defaultUrl"],
        "jupyter_root_dir": spec["jupyterServer"]["rootDir"],
        "jupyter_token": spec["auth"]["token"],
        # Routing
        "host": spec["routing"]["host"],
        "path": spec["routing"]["path"].rstrip("/"),
        "full_url": urljoin(
            f"https://{spec['routing']['host']}", spec["routing"]["path"].rstrip("/")
        ),
        # Volume
        "volume_size": spec["volume"]["size"],
        "volume_storage_class": spec["volume"]["storageClass"],
    }
    if spec["auth"]["oidc"]["enabled"]:
        template_values.update(
            {
                "oidc_issuer_url": spec["auth"]["oidc"]["issuerUrl"],
                "oidc_client_id": spec["auth"]["oidc"]["clientId"],
                "oidc_client_secret": spec["auth"]["oidc"]["clientSecret"],
                "oidc_user_id": spec["auth"]["oidc"]["userId"],
                "authentication_cookie_secret": base64.urlsafe_b64encode(
                    os.urandom(32)
                ).decode(),
            }
        )
    return template_values


def render_template(template_file, template_values):
    """
    Render a template given the template strings and return
    a python dictionary specifying the resource.
    """

    tmpl_path = os.path.join(TEMPLATE_DIR, template_file)
    tmpl_string = open(tmpl_path, "rt").read()
    yaml_string = tmpl_string.format(**template_values)
    resource_spec = yaml.safe_load(yaml_string)
    return resource_spec


def get_resources_specs(metadata, spec):
    """
    Create the resource specifications (as nested python dictionaries) that
    make up the custom resource object. No input validation happens here, we
    rely on CRD schema validation.
    """

    template_values = create_template_values(metadata, spec)

    resource_configs = get_resource_configs(
        oidc_enabled=spec["auth"]["oidc"]["enabled"], api_only=False
    )

    # Create a list of the resources to be created (as dictionaries)
    resources_specs = {}
    for key, config in resource_configs.items():
        resources_specs[key] = render_template(config["template"], template_values)

    # Adapt traefik rules to non-oidc case
    cm_data = resources_specs["configmap"]["data"]
    if not spec["auth"]["oidc"]["enabled"]:
        proxy_middlewares = cm_data["proxy-rules.yaml"]["http"]["routers"]["proxy"][
            "middlewares"
        ]
        proxy_middlewares.remove("oidcPlugin")
        proxy_middlewares.remove("customAuthorization")
        del cm_data["oidc-plugin-rules.yaml"]

    # Flatten the traefik configuration dictionaries into a string
    for key, value in cm_data.items():
        cm_data[key] = yaml.safe_dump(value)

    # Go through the list of modifications and apply them to the right resource.
    # Note that deepmerge modifies the resource_dict in-place
    for mod in spec["resourceModifications"]:
        always_merger.merge(resources_specs[mod["resource"]], mod["modification"])

    container_keys = [
        key for key, config in resource_configs.items() if not "api" in config
    ]

    for container_key in container_keys:
        resources_specs["statefulset"]["spec"]["template"]["spec"]["containers"].append(
            resources_specs[container_key]
        )
        resources_specs.pop(container_key, None)

    return resources_specs
