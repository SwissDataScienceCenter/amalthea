import json
import os
import yaml

api_group = os.getenv("CRD_API_GROUP", "amalthea.dev")
api_version = os.getenv("CRD_API_VERSION", "v1alpha1")
custom_resource_name = os.getenv("CRD_NAME", "JupyterServer")

# Strings which will be evaluated as true on env variables.
TRUE_STRINGS = ["True", "true", "1"]

# Note: This is an experimental feature, us it with care.
reschedule_on_node_failure = (
    os.getenv("RESCHEDULE_ON_NODE_FAILURE", False) in TRUE_STRINGS
)

try:
    with open("/app/config/kopf-operator-settings.yaml", "r") as f:
        kopf_operator_settings = yaml.safe_load(f.read())
except FileNotFoundError:
    kopf_operator_settings = {}

amalthea_selector_labels = yaml.safe_load(os.getenv("AMALTHEA_SELECTOR_LABELS", "{}"))


# Allowed child resources / groups that we need per default
CHILD_RESOURCES = [
    {"name": "statefulsets", "group": "apps"},
    {"name": "pods", "group": ""},
    {"name": "ingresses", "group": "networking.k8s.io"},
    {"name": "secrets", "group": ""},
    {"name": "configmaps", "group": ""},
    {"name": "services", "group": ""},
    {"name": "persistentvolumeclaims", "group": ""},
]

CHILD_RESOURCES += json.loads(os.getenv("EXTRA_CHILD_RESOURCES", "[]"))

KOPF_CREATE_TIMEOUT = None if os.getenv("KOPF_CREATE_TIMEOUT") is None else float(os.getenv("KOPF_CREATE_TIMEOUT"))
KOPF_CREATE_BACKOFF = None if os.getenv("KOPF_CREATE_BACKOFF") is None else float(os.getenv("KOPF_CREATE_BACKOFF"))
KOPF_CREATE_RETRIES = None if os.getenv("KOPF_CREATE_RETRIES") is None else int(os.getenv("KOPF_CREATE_RETRIES"))