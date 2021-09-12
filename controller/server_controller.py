from expiringdict import ExpiringDict
import kopf
import kubernetes.client as k8s_client
from kubernetes.dynamic import DynamicClient
from kubernetes.dynamic.exceptions import NotFoundError
from datetime import datetime
import pytz

from controller import config
from controller.k8s_resources import CONTENT_TYPES, get_children_specs, get_urls
from controller.culling import get_cpu_usage, get_js_server_status


# A dictionary matching a K8s event type to a jsonpatch operation type
JSONPATCH_OPS = {"MODIFIED": "replace", "ADDED": "add", "DELETED": "remove"}

# A very simple in-memory cache to store the result of the
# "resources" query of the dynamic API client.
api_cache = ExpiringDict(max_len=100, max_age_seconds=60)


PARENT_UID_LABEL_KEY = f"{config.api_group}/parent-uid"
PARENT_NAME_LABEL_KEY = f"{config.api_group}/parent-name"
CHILD_KEY_LABEL_KEY = f"{config.api_group}/child-key"
MAIN_POD_LABEL_KEY = f"{config.api_group}/main-pod"


def get_labels(
    parent_name, parent_uid, parent_labels, child_key=None, is_main_pod=False
):
    """Create the appropriate labels per resource"""
    # Add labels from lowest to highest priority
    labels = {}
    labels.update(parent_labels)
    labels.update(config.amalthea_selector_labels)
    labels.update(
        {
            "app.kubernetes.io/component": config.custom_resource_name.lower(),
            PARENT_UID_LABEL_KEY: parent_uid,
            PARENT_NAME_LABEL_KEY: parent_name,
        }
    )
    if child_key:
        labels.update({CHILD_KEY_LABEL_KEY: child_key})
    if is_main_pod:
        labels.update({MAIN_POD_LABEL_KEY: "true"})
    return labels


def get_api(api_version, kind):
    """
    Get the proper API for a certain resource. We cache the resources
    availabe in the cluster for 60 seconds in order to reduce the amount
    of unnecessary requests in busy clusters.
    """
    try:
        return api_cache[(api_version, kind)]
    except KeyError:
        client = DynamicClient(k8s_client.ApiClient())
        api_cache[(api_version, kind)] = client.resources.get(
            api_version=api_version, kind=kind
        )
        return api_cache[(api_version, kind)]


def create_namespaced_resource(namespace, body):
    """
    Create a k8s resource given the namespace and the full resource object.
    """
    api = get_api(body["apiVersion"], body["kind"])
    return api.create(namespace=namespace, body=body)


@kopf.on.startup()
def configure(logger, settings, **_):
    """
    Configure the operator - see https://kopf.readthedocs.io/en/stable/configuration/
    for options.
    """

    if config.kopf_operator_settings:
        try:
            for key, val in config.kopf_operator_settings.items():
                getattr(settings, key).__dict__.update(val)
        except AttributeError as e:
            logger.error(f"Problem when configuring the Operator: {e}")


@kopf.on.create(
    config.api_group,
    config.api_version,
    config.custom_resource_name,
    timeout=config.KOPF_CREATE_TIMEOUT,
    retries=config.KOPF_CREATE_RETRIES,
    backoff=config.KOPF_CREATE_BACKOFF,
)
def create_fn(labels, logger, name, namespace, spec, uid, **_):
    """
    Watch the creation of jupyter server objects and create all
    the necessary k8s child resources which make the actual jupyter server.
    """
    children_specs = get_children_specs(name, spec, logger)

    # We make sure the pod created from the statefulset gets labeled
    # with the custom resource references and add a special label to
    # distinguish it from direct children.
    kopf.label(
        children_specs["statefulset"]["spec"]["template"],
        labels=get_labels(name, uid, labels, is_main_pod=True),
    )

    # Add the labels to all child resources and create them in the cluster
    children_uids = {}

    for child_key, child_spec in children_specs.items():
        # TODO: look at the option of using subhandlers here.
        kopf.label(
            child_spec,
            labels=get_labels(name, uid, labels, child_key=child_key),
        )
        kopf.adopt(child_spec)

        children_uids[child_key] = create_namespaced_resource(
            namespace=namespace, body=child_spec
        ).metadata.uid

    return {"createdResources": children_uids, "fullServerURL": get_urls(spec)[1]}


def cull_idle_jupyter_servers(body, name, namespace, logger, **kwargs):
    """
    Check if a session is idle (has zero open connections in proxy and CPU is below
    threshold). If the session is idle then annotate the jupyterserver resource with
    the idle duration. If any sessions have been idle for long enough, then cull them.
    """
    try:
        pod_name = body["status"]["mainPod"]["name"]
    except KeyError:
        return
    cpu_usage = get_cpu_usage(pod=pod_name, namespace=namespace)
    js_server_status = get_js_server_status(body)
    custom_resource_api = get_api(config.api_version, config.custom_resource_name)
    idle_seconds = int(body["status"].get("idleSeconds", 0))
    logger.info(
        f"Checking idle status of session {name}, "
        f"idle seconds: {idle_seconds}, "
        f"cpu usage: {cpu_usage}m, "
        f"server status: {js_server_status}"
    )

    now = pytz.UTC.localize(datetime.utcnow())
    jupyter_server_is_idle = (
        cpu_usage <= config.CPU_USAGE_MILLICORES_IDLE_THRESHOLD
        and type(js_server_status) is dict
        and js_server_status.get("connections", 0) == 0
        and js_server_status.get("kernels", 0) == 0
        and (
            now - js_server_status.get("last_activity", now).astimezone(pytz.UTC)
        ).total_seconds()
        > config.JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS
    )

    if (
        jupyter_server_is_idle
        and idle_seconds + config.JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS
        >= config.JUPYTER_SERVER_CULL_IDLE_THRESHOLD_SECONDS
    ):
        logger.info(f"Deleting Jupyter server {name} due to inactivity")
        try:
            custom_resource_api.delete(name=name, namespace=namespace)
        except NotFoundError:
            pass
        return

    if jupyter_server_is_idle:
        logger.info(
            f"Jupyter Server {name} in namespace {namespace} found to be idle for "
            f"{idle_seconds + config.JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS}"
        )
        try:
            custom_resource_api.patch(
                namespace=namespace,
                name=name,
                body={
                    "status": {
                        "idleSeconds": str(
                            idle_seconds
                            + config.JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS
                        ),
                    },
                },
                content_type=CONTENT_TYPES["merge-patch"],
            )
        except NotFoundError:
            pass
    else:
        if idle_seconds > 0:
            try:
                logger.info(f"Resetting idle timer for Jupyter server {name} in namespace {namespace}.")
                custom_resource_api.patch(
                    namespace=namespace,
                    name=name,
                    body={
                        "status": {"idleSeconds": "0"},
                    },
                    content_type=CONTENT_TYPES["merge-patch"],
                )
            except NotFoundError:
                pass


# create @kopf.on.event(...) type of decorators
# Go to the bottom of the update_status function definition to see how
# those decorators are applied.
def get_update_decorator(child_resource_kind):
    return kopf.on.event(
        child_resource_kind["name"],
        group=child_resource_kind["group"],
        labels={PARENT_NAME_LABEL_KEY: kopf.PRESENT},
    )


def update_status(body, event, labels, logger, meta, name, namespace, uid, **_):
    """
    Update the custom object status with the status of all children
    and the statefulsets pod as only grand child.
    """

    logger.info(f"{body['kind']}: {event['type']}")

    # Collect labels and other metainformation from the resource which
    # triggered the event.
    parent_name = labels[PARENT_NAME_LABEL_KEY]
    parent_uid = labels[PARENT_UID_LABEL_KEY]
    child_key = labels.get(CHILD_KEY_LABEL_KEY, None)
    owner_references = meta.get("ownerReferences", [])
    owner_uids = [ref["uid"] for ref in owner_references]
    is_main_pod = labels.get(MAIN_POD_LABEL_KEY, "") == "true"

    # Check if the jupyter server is the actual parent (ie owner) in order
    # to exclude the grand children of the jupyter server. The only grand child
    # resource we're hanlding here is the statefulset pod.
    if (parent_uid not in owner_uids) and not is_main_pod:
        logger.info(
            f"Ignoring event for non-child resource of \
            kind {event['type']} on resource of {body['kind']}"
        )
        return

    # Assemble the jsonpatch to update the custom objects status
    try:
        op = JSONPATCH_OPS[event["type"]]
    except KeyError:
        logger.info(
            f"Ignoring event of kind {event['type']} on resource of {body['kind']}"
        )
        return

    path = "/status/mainPod" if is_main_pod else f"/status/children/{child_key}"
    value = {
        "uid": uid,
        "name": name,
        "kind": body["kind"],
        "apiVersion": body["apiVersion"],
        "status": body.get("status", None),
    }
    patch_op = {"op": op, "path": path}
    if op in ["add", "replace"]:
        patch_op["value"] = value

    # We use the dynamic client for patching since we need
    # content_type="application/json-patch+json"
    custom_resource_api = get_api(config.api_version, config.custom_resource_name)
    try:
        custom_resource_api.patch(
            namespace=namespace,
            name=parent_name,
            body=[patch_op],
            content_type=CONTENT_TYPES["json-patch"],
        )
    # Handle the case when the custom resource is already gone, must
    # happen for removals of children exclusively, not for "add" or "replace".
    except NotFoundError as e:
        if op == "remove":
            pass
        else:
            raise e


# Add the actual decorators
for child_resource_kind in config.CHILD_RESOURCES:
    update_status = get_update_decorator(child_resource_kind)(update_status)

# Add culling if enabled
if config.CULLING_ENABLED:
    kopf.timer(
        config.api_group,
        config.api_version,
        config.custom_resource_name,
        interval=config.JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS,
    )(cull_idle_jupyter_servers)
