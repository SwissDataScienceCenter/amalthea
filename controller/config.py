import json
import os
import yaml

from controller.utils import sanitize_prometheus_metric_label_name

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
JUPYTER_SERVER_PENDING_CHECK_INTERVAL_SECONDS = int(
    os.getenv("JUPYTER_SERVER_PENDING_CHECK_INTERVAL_SECONDS", 300)
)
CPU_USAGE_MILLICORES_IDLE_THRESHOLD = int(
    os.getenv("CPU_USAGE_MILLICORES_IDLE_THRESHOLD", 200)
)
SERVER_SCHEDULER_NAME = os.getenv("SERVER_SCHEDULER_NAME", None)
JUPYTER_SERVER_RESOURCE_CHECK_ENABLED = (
    os.getenv("JUPYTER_SERVER_RESOURCE_CHECK_ENABLED", "true").lower() == "true"
)
JUPYTER_SERVER_RESOURCE_CHECK_INTERVAL_SECONDS = int(
    os.getenv("JUPYTER_SERVER_RESOURCE_CHECK_INTERVAL_SECONDS", 30)
)

# A dictionary matching a K8s event type to a jsonpatch operation type
JSONPATCH_OPS = {"MODIFIED": "replace", "ADDED": "add", "DELETED": "remove"}

PARENT_UID_LABEL_KEY = f"{api_group}/parent-uid"
PARENT_NAME_LABEL_KEY = f"{api_group}/parent-name"
CHILD_KEY_LABEL_KEY = f"{api_group}/child-key"
MAIN_POD_LABEL_KEY = f"{api_group}/main-pod"

METRICS_ENABLED = os.environ.get("METRICS_ENABLED", "false").lower() == "true"
METRICS_EXTRA_LABELS = json.loads(os.environ.get("METRICS_EXTRA_LABELS", "[]"))
METRICS_EXTRA_LABELS_SANITIZED = tuple([
    sanitize_prometheus_metric_label_name(i) for i in METRICS_EXTRA_LABELS
])
METRICS_PORT = int(os.environ.get("METRICS_PORT", 8765))

"""How long should a session be unschedulable after it has been created
before it is marked as failed. All session pods are initially unschedulable for a short duration.
This occurs mostly because PV provisioning takes time or because of other reasons.
Therefore, a session should not fail for being unschedulable right after it is created.
If this is set to zero then the session status usually goes from
Starting -> Failed (for a few seconds) -> Starting -> Running."""
UNSCHEDULABLE_FAILURE_THRESHOLD_SECONDS = int(
    os.environ.get("UNSCHEDULABLE_FAILURE_THRESHOLD_SECONDS", 60)
)
