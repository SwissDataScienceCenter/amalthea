import base64
import jsonpatch
import json_merge_patch
import logging
import os
import yaml
import json
from urllib.parse import urljoin


TEMPLATE_DIR = os.path.join(os.path.dirname(__file__), "templates")


def get_resource_list(pvc_enabled=False):
    """
    Define a list of all resources that should be created.
    """
    resources = [
        {
            "name": "service",
            "template": "service.yaml",
        },
        {
            "name": "ingress",
            "template": "ingress.yaml",
        },
        {
            "name": "statefulset",
            "template": "statefulset.yaml",
        },
        {
            "name": "configmap",
            "template": "configmap.yaml",
        },
    ]

    if pvc_enabled:
        resources.append(
            {
                "name": "pvc",
                "template": "pvc.yaml",
            }
        )
    return resources


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
        # Session ingress
        "ingress_tls_secret": spec["routing"]["tlsSecret"],
        "ingress_annotations": spec["routing"]["ingressAnnotations"],
        # Cookie cleaner
        "cookie_whitelist": json.dumps(spec["auth"]["cookieWhiteList"]),
        "cookie_blacklist": json.dumps(spec["auth"].get("cookieBlackList", None)),
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
    if spec["storage"]["pvc"]["enabled"]:
        template_values.update({"volume_size": spec["storage"]["size"]})
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


def get_resources(metadata, spec):
    """
    Create the resource specifications (as nested python dictionaries) that
    make up the custom resource object. No input validation happens here, we
    rely on CRD schema validation.
    """

    template_values = create_template_values(metadata, spec)

    resources = get_resource_list(
        pvc_enabled=spec["storage"]["pvc"]["enabled"],
    )

    # Create a list of the resources to be created (as dictionaries)
    resources_specs = {}
    for resource in resources:
        resources_specs[resource["name"]] = render_template(
            resource["template"], template_values
        )

    # Add pvc or emptyDir to statefulset volumes
    if spec["storage"]["pvc"]["enabled"]:
        resources_specs["statefulset"]["spec"]["template"]["spec"]["volumes"].append(
            {
                "name": "workspace",
                "persistentVolumeClaim": {"claimName": metadata["name"]},
            }
        )
        # If the storage class is provided update the manifests, else without specifying
        # anything the default storage class is used automatically
        if spec["storage"]["pvc"].get("storageClassName") is not None:
            resources_specs["pvc"]["spec"]["storageClassName"] = spec["storage"][
                "pvc"
            ].get("storageClassName")
    else:
        resources_specs["statefulset"]["spec"]["template"]["spec"]["volumes"].append(
            {"name": "workspace", "emptyDir": {"sizeLimit": spec["storage"]["size"]}}
        )

    # Adapt statefulset containers for OIDC being case
    if spec["auth"]["oidc"]["enabled"]:
        for template_file in [
            "authentication-plugin.yaml",
            "authorization-plugin.yaml",
        ]:
            resources_specs["statefulset"]["spec"]["template"]["spec"][
                "containers"
            ].append(render_template(template_file, template_values))

    # Adapt traefik rules for non-OIDC case.
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

    # Finally, apply all the patches and return the result
    # TODO: Enable strategic merge patches if possible
    for patch in spec["patches"]:
        if patch["type"] == "jsonPatch":
            resources_specs = jsonpatch.apply_patch(
                resources_specs, json.dumps(patch["patch"])
            )
        elif patch["type"] == "jsonMergePatch":
            resources_specs = json_merge_patch.merge(resources_specs, patch["patch"])
        else:
            # This should actually already be caught at the CRD validation level.
            logging.debug(
                f"Invalid patch type - ignoring this patch: {json.dumps(patch)}"
            )

    return resources_specs
