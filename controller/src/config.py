import json
import os
import yaml

api_group = "renku.io"
api_version = "v1alpha1"
custom_resource_name = "JupyterServer"

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
