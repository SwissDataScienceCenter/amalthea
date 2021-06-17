import base64
import json
import os
from urllib.parse import urljoin

import jsonpatch
import json_merge_patch
import yaml


CONTENT_TYPES = {
    "json-patch": "application/json-patch+json",
    "merge-patch": "application/merge-patch+json",
}
TEMPLATE_DIR = os.path.join(os.path.dirname(__file__), "templates")


def get_children_templates(pvc_enabled=False):
    """
    Define a list of all resources that should be created.
    """
    children_templates = {
        "service": "service.yaml",
        "ingress": "ingress.yaml",
        "statefulset": "statefulset.yaml",
        "configmap": "configmap.yaml",
    }
    if pvc_enabled:
        children_templates["pvc"] = "pvc.yaml"

    return children_templates


def create_template_values(auth, jupyter_server, name, oidc, routing, pvc, storage):
    """
    Create a single non-nested dictionary which contains all the
    variables needed for the templating as keys.
    """

    template_values = {
        # Metadata
        "name": name,
        # Jupyter server
        "jupyter_image": jupyter_server["image"],
        "jupyter_default_url": jupyter_server["defaultUrl"],
        "jupyter_root_dir": jupyter_server["rootDir"],
        "jupyter_token": auth["token"],
        # Routing
        "host": routing["host"],
        "path": routing["path"].rstrip("/"),
        "full_url": urljoin(
            f"http{'s' if routing['tls']['enabled'] else ''}://{routing['host']}",
            routing["path"].rstrip("/"),
        ),
        # Session ingress
        # TLS secret and annotations will be removed from ingress if ""
        "ingress_tls_secret": routing["tls"].get("secret", ""),
        "ingress_annotations": routing["ingressAnnotations"],
        # Cookie cleaner
        "cookie_whitelist": json.dumps(auth["cookieWhiteList"]),
        "cookie_blacklist": json.dumps(auth.get("cookieBlackList", None)),
    }
    if oidc["enabled"]:
        template_values.update(
            {
                "oidc_issuer_url": oidc["issuerUrl"],
                "oidc_client_id": oidc["clientId"],
                "oidc_client_secret": oidc["clientSecret"],
                "oidc_user_id": oidc["userId"],
                "authentication_cookie_secret": base64.urlsafe_b64encode(
                    os.urandom(32)
                ).decode(),
            }
        )
    if pvc["enabled"]:
        template_values.update({"volume_size": storage["size"]})
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


def get_children_specs(name, spec, logger):
    """
    Create the resource specifications (as nested python dictionaries) that
    make up the custom resource object. No input validation happens here, we
    rely on CRD schema validation.
    """

    # Deeply nested python dictionaries are annoying...
    auth = spec["auth"]
    jupyter_server = spec["jupyterServer"]
    oidc = auth["oidc"]
    routing = spec["routing"]
    storage = spec["storage"]
    pvc = storage["pvc"]

    template_values = create_template_values(
        auth=auth,
        jupyter_server=jupyter_server,
        name=name,
        oidc=oidc,
        routing=routing,
        pvc=pvc,
        storage=storage,
    )

    # Generate one big dictionary containing the specs of all child
    # resources to be created.
    children_templates = get_children_templates(
        pvc_enabled=pvc["enabled"],
    )
    children_specs = {
        key: render_template(tpl, template_values)
        for key, tpl in children_templates.items()
    }

    pod_spec = children_specs["statefulset"]["spec"]["template"]["spec"]

    # TODO: We have to do more and more modifications here which could be avoided
    # TODO: by choosing a proper templating language like jinja.

    if not routing["tls"]["enabled"]:
        del children_specs["ingress"]["spec"]["tls"]

    # Add pvc or emptyDir to statefulset volumes
    if pvc["enabled"]:
        pod_spec["volumes"].append(
            {
                "name": "workspace",
                "persistentVolumeClaim": {
                    "claimName": children_specs["pvc"]["metadata"]["name"]
                },
            }
        )
        # If the storage class is provided update the manifests, else without specifying
        # anything the default storage class is used automatically.
        if "storageClassName" in pvc:
            children_specs["pvc"]["spec"]["storageClassName"] = pvc["storageClassName"]
    else:
        pod_spec["volumes"].append(
            {"name": "workspace", "emptyDir": {"sizeLimit": storage["size"]}}
        )

    # Adapt statefulset containers for the OIDC case (ie add two containers that
    # will serve a forward-auth middlewares)
    if oidc["enabled"]:
        for template_file in [
            "authentication-plugin.yaml",
            "authorization-plugin.yaml",
        ]:
            pod_spec["containers"].append(
                render_template(template_file, template_values)
            )

    # Adapt traefik rules for the non-OIDC case (ie remove the
    # two forward-auth middlewares)
    cm_data = children_specs["configmap"]["data"]
    if not oidc["enabled"]:
        proxy_middlewares = cm_data["proxy-rules.yaml"]["http"]["routers"]["proxy"][
            "middlewares"
        ]
        proxy_middlewares.remove("oidcPlugin")
        proxy_middlewares.remove("customAuthorization")
        del cm_data["oidc-plugin-rules.yaml"]

    # Serialize the traefik configuration dictionary into a string
    for key, value in cm_data.items():
        cm_data[key] = yaml.safe_dump(value)

    # Finally, apply all the patches and return the result
    # TODO: Enable strategic merge patches if possible
    for patch in spec["patches"]:
        if patch["type"] == CONTENT_TYPES["json-patch"]:
            children_specs = jsonpatch.apply_patch(
                children_specs, json.dumps(patch["patch"])
            )
        elif patch["type"] == CONTENT_TYPES["merge-patch"]:
            children_specs = json_merge_patch.merge(children_specs, patch["patch"])
        else:
            # This should actually already be caught at the CRD validation level.
            logger.debug(
                f"Invalid patch type - ignoring this patch: {json.dumps(patch)}"
            )

    return children_specs
