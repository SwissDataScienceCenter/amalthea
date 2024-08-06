import json
import os
import yaml

from controller.config_types import AuditlogConfig, PrometheusMetricsConfig

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

KOPF_CREATE_TIMEOUT = None if os.getenv("KOPF_CREATE_TIMEOUT", "") == "" else float(os.getenv("KOPF_CREATE_TIMEOUT"))
KOPF_CREATE_BACKOFF = None if os.getenv("KOPF_CREATE_BACKOFF", "") == "" else float(os.getenv("KOPF_CREATE_BACKOFF"))
KOPF_CREATE_RETRIES = None if os.getenv("KOPF_CREATE_RETRIES", "") == "" else int(os.getenv("KOPF_CREATE_RETRIES"))

JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS = int(os.getenv("JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS", 300))
JUPYTER_SERVER_PENDING_CHECK_INTERVAL_SECONDS = int(os.getenv("JUPYTER_SERVER_PENDING_CHECK_INTERVAL_SECONDS", 300))
CPU_USAGE_MILLICORES_IDLE_THRESHOLD = int(os.getenv("CPU_USAGE_MILLICORES_IDLE_THRESHOLD", 200))
SERVER_SCHEDULER_NAME = os.getenv("SERVER_SCHEDULER_NAME", None)
JUPYTER_SERVER_RESOURCE_CHECK_ENABLED = os.getenv("JUPYTER_SERVER_RESOURCE_CHECK_ENABLED", "true").lower() == "true"
JUPYTER_SERVER_RESOURCE_CHECK_INTERVAL_SECONDS = int(os.getenv("JUPYTER_SERVER_RESOURCE_CHECK_INTERVAL_SECONDS", 30))

# A dictionary matching a K8s event type to a jsonpatch operation type
JSONPATCH_OPS = {"MODIFIED": "replace", "ADDED": "add", "DELETED": "remove"}

PARENT_UID_LABEL_KEY = f"{api_group}/parent-uid"
PARENT_NAME_LABEL_KEY = f"{api_group}/parent-name"
CHILD_KEY_LABEL_KEY = f"{api_group}/child-key"
MAIN_POD_LABEL_KEY = f"{api_group}/main-pod"

METRICS: PrometheusMetricsConfig = PrometheusMetricsConfig.dataconf_from_env()
AUDITLOG: AuditlogConfig = AuditlogConfig.dataconf_from_env()

# NOTE: K8s attemts restarts with a exponentially increasing delay
# so a restart limit of 5 for example takes some 10 mins to go through
# and show a failed status. This is because every subsequent restart
# has a longer exponentially increasing delay and the container is not
# considered failed until it exceeds the restart limits below
JUPYTER_SERVER_INIT_CONTAINER_RESTART_LIMIT: int = int(os.environ.get("JUPYTER_SERVER_INIT_CONTAINER_RESTART_LIMIT", 1))
JUPYTER_SERVER_CONTAINER_RESTART_LIMIT: int = int(os.environ.get("JUPYTER_SERVER_CONTAINER_RESTART_LIMIT", 3))

QUOTA_EXCEEDED_MESSAGE = "The resource quota has been exceeded."

CLUSTER_WIDE: bool = os.getenv("CLUSTER_WIDE", "false").lower() == "true"
_namespaces_raw: str | None = os.getenv("NAMESPACES")
NAMESPACES: list[str] = (
    _namespaces_raw.split(",")
    if isinstance(_namespaces_raw, str) and len(_namespaces_raw) > 0
    else []
)
VERBOSE = os.environ.get("VERBOSE", "false").lower() == "true"
DEBUG = os.environ.get("DEBUG", "false").lower() == "true"
