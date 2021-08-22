import base64
import json
import os
from urllib.parse import urljoin

import jinja2
import jsonpatch
import json_merge_patch
import yaml


CONTENT_TYPES = {
    "json-patch": "application/json-patch+json",
    "merge-patch": "application/merge-patch+json",
}
TEMPLATE_DIR = os.path.join(os.path.dirname(__file__), "templates")


def get_urls(spec):
    """Small convenience method to construct the URLs of the Jupyter server."""
    host_url = f"http{'s' if spec['routing']['tls']['enabled'] else ''}://{spec['routing']['host']}"
    full_url = urljoin(
        host_url,
        spec["routing"]["path"].rstrip("/"),
    )
    return host_url, full_url


def get_children_templates(pvc_enabled=False):
    """
    Define a list of all resources that should be created.
    """
    children_templates = {
        "service": "service.yaml",
        "ingress": "ingress.yaml",
        "statefulset": "statefulset.yaml",
        "configmap": "configmap.yaml",
        "secret": "secret.yaml",
    }
    if pvc_enabled:
        children_templates["pvc"] = "pvc.yaml"

    return children_templates


def create_template_values(name, spec):
    """
    Create a single non-nested dictionary which contains all the
    variables needed for the templating as keys because too much logic
    or deeply nested python dictionaries in templates are not fun...
    """

    host_url, full_url = get_urls(spec)
    # All we need for template rendering, alphabetically listed
    template_values = {
        "auth": spec["auth"],
        "authentication_plugin_cookie_secret": base64.urlsafe_b64encode(
            os.urandom(32)
        ).decode(),
        "full_url": full_url,
        "host_url": host_url,
        "ingress_annotations": json.dumps(spec["routing"]["ingressAnnotations"]),
        "jupyter_server": spec["jupyterServer"],
        "jupyter_server_app_token": spec["auth"].get("token", os.urandom(32).hex()),
        "jupyter_server_cookie_secret": os.urandom(32).hex(),
        "name": name,
        "oidc": spec["auth"]["oidc"],
        "path": spec["routing"]["path"].rstrip("/"),
        "pvc": spec["storage"]["pvc"],
        "routing": spec["routing"],
        "storage": spec["storage"],
    }

    return template_values


def render_template(template_file, template_values):
    """
    Render a template given the template strings and return
    a python dictionary specifying the resource.
    """
    import base64

    tmpl_loader = jinja2.FileSystemLoader(TEMPLATE_DIR)
    tmpl_env = jinja2.Environment(loader=tmpl_loader)
    tmpl_env.filters["b64encode"] = lambda x: base64.b64encode(
        x.encode("utf-8")
    ).decode("ascii")
    yaml_string = tmpl_env.get_template(template_file).render(**template_values)
    resource_spec = yaml.safe_load(yaml_string)
    return resource_spec


def get_children_specs(name, spec, logger):
    """
    Create the resource specifications (as nested python dictionaries) that
    make up the custom resource object. No input validation happens here, we
    rely on CRD schema validation.
    """

    template_values = create_template_values(name, spec)

    # Generate one big dictionary containing the specs of all child
    # resources to be created.
    children_templates = get_children_templates(
        pvc_enabled=spec["storage"]["pvc"]["enabled"],
    )
    children_specs = {
        key: render_template(tpl, template_values)
        for key, tpl in children_templates.items()
    }

    # Apply all the patches and return the result
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
