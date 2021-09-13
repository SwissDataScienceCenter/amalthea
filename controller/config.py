import json
import os
import yaml

api_group = os.getenv("CRD_API_GROUP", "amalthea.dev")
api_version = os.getenv("CRD_API_VERSION", "v1alpha1")
custom_resource_name = os.getenv("CRD_NAME", "JupyterServer")


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

KOPF_CREATE_TIMEOUT = (
    None
    if os.getenv("KOPF_CREATE_TIMEOUT", "") == ""
    else float(os.getenv("KOPF_CREATE_TIMEOUT"))
)
KOPF_CREATE_BACKOFF = (
    None
    if os.getenv("KOPF_CREATE_BACKOFF", "") == ""
    else float(os.getenv("KOPF_CREATE_BACKOFF"))
)
KOPF_CREATE_RETRIES = (
    None
    if os.getenv("KOPF_CREATE_RETRIES", "") == ""
    else int(os.getenv("KOPF_CREATE_RETRIES"))
)

JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS = int(
    os.getenv("JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS", 300)
)
CPU_USAGE_MILLICORES_IDLE_THRESHOLD = int(
    os.getenv("CPU_USAGE_MILLICORES_IDLE_THRESHOLD", 200)
)
