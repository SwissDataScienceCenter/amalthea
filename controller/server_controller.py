import json
import logging
from typing import Dict

import kopf
from datetime import datetime, timezone

import kubernetes.client
from kubernetes.client.models import V1DeleteOptions
from kubernetes.dynamic.exceptions import NotFoundError
from prometheus_client import start_http_server
import pytz

from controller import config
from controller.k8s_resources import CONTENT_TYPES, get_children_specs, get_urls
from controller.culling import get_cpu_usage_for_culling, get_js_server_status
from controller.utils import (
    get_pod_metrics,
    get_volume_disk_capacity,
    get_api,
    parse_pod_metrics,
)
from controller.metrics.s3 import S3MetricHandler, S3RotatingLogHandler, S3Formatter
from controller.metrics.prometheus import PrometheusMetricHandler
from controller.metrics.events import MetricEvent
from controller.metrics.queue import MetricsQueue
from controller.server_status_enum import ServerStatusEnum
from controller.server_status import ServerStatus


metric_handlers = []
if config.METRICS.enabled:
    metric_handlers.append(PrometheusMetricHandler(config.METRICS.extra_labels))
if config.AUDITLOG.enabled:
    s3_metric_logger = logging.getLogger("s3")
    s3_logging_handler = S3RotatingLogHandler(
        "/tmp/amalthea_audit_log.txt", "a", config.AUDITLOG.s3
    )
    s3_logging_handler.setFormatter(S3Formatter())
    s3_metric_logger.addHandler(s3_logging_handler)
    metric_handlers.append(S3MetricHandler(s3_metric_logger, config.AUDITLOG))
metric_events_queue = MetricsQueue(metric_handlers)


def patch_jupyter_servers(
    *, name: str, namespace: str, body: Dict, custom_resource_api=None
) -> bool:
    """Patch a server with the given body."""
    custom_resource_api = custom_resource_api or get_api(
        config.api_version, config.custom_resource_name, config.api_group
    )

    try:
        custom_resource_api.patch(
            namespace=namespace,
            name=name,
            body=body,
            content_type=CONTENT_TYPES["merge-patch"],
        )
    except NotFoundError:
        return False
    else:
        return True


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
            config.PARENT_UID_LABEL_KEY: parent_uid,
            config.PARENT_NAME_LABEL_KEY: parent_name,
        }
    )
    if child_key:
        labels.update({config.CHILD_KEY_LABEL_KEY: child_key})
    if is_main_pod:
        labels.update({config.MAIN_POD_LABEL_KEY: "true"})
    return labels


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
    settings.posting.level = logging.INFO

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
def create_fn(labels, logger, name, namespace, spec, uid, body, **_):
    """
    Watch the creation of jupyter server objects and create all
    the necessary k8s child resources which make the actual jupyter server.
    """
    api = get_api(config.api_version, config.custom_resource_name, config.api_group)
    now = pytz.UTC.localize(datetime.utcnow()).isoformat(timespec="seconds")
    try:
        api.patch(
            namespace=namespace,
            name=name,
            body={
                "metadata": {
                    "annotations": {
                        "renku.io/last-activity-date": now,
                    },
                },
                "status": {
                    "state": ServerStatusEnum.Starting.value,
                    "startingSince": now,
                },
            },
            content_type=CONTENT_TYPES["merge-patch"],
        )
    except NotFoundError:
        pass

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


@kopf.on.delete(config.api_group, config.api_version, config.custom_resource_name)
def delete_fn(labels, body, namespace, name, **_):
    """
    The jupyter server has been deleted.
    """
    api = get_api(config.api_version, config.custom_resource_name, config.api_group)
    new_status = ServerStatusEnum.Stopping

    api.patch(
        namespace=namespace,
        name=name,
        body={
            "status": {
                "state": new_status.value,
            },
        },
        content_type=CONTENT_TYPES["merge-patch"],
    )


@kopf.on.event(
    version=config.api_version, kind=config.custom_resource_name, group=config.api_group
)
def update_server_state(body, namespace, name, **_):
    server_status = ServerStatus.from_server_spec(
        body,
        config.JUPYTER_SERVER_INIT_CONTAINER_RESTART_LIMIT,
        config.JUPYTER_SERVER_CONTAINER_RESTART_LIMIT,
    )
    new_status = server_status.overall_status
    old_status_raw = body.get("status", {}).get("state")
    old_status = ServerStatusEnum.from_string(old_status_raw)
    new_summary = server_status.get_container_summary()
    old_summary = body.get("status", {}).get("containerStates", {})
    # NOTE: Updating the status for deletions is handled in a specific delete handler
    if (
        (old_status != new_status or new_summary != old_summary)
        and new_status != ServerStatusEnum.Stopping
    ):
        now = pytz.UTC.localize(datetime.utcnow())
        api = get_api(
            config.api_version, config.custom_resource_name, config.api_group
        )
        try:
            api.patch(
                namespace=namespace,
                name=name,
                body={
                    "status": {
                        "state": new_status.value,
                        "containerStates": new_summary,
                        "failedSince": (
                            now.isoformat() if new_status == ServerStatusEnum.Failed else None
                        ),
                    },
                },
                content_type=CONTENT_TYPES["merge-patch"],
            )
        except NotFoundError:
            pass


@kopf.on.field(
    group=config.api_group,
    version=config.api_version,
    kind=config.custom_resource_name,
    field="spec.jupyterServer.hibernated",
)
def hibernation_field_handler(body, logger, name, namespace, **_):
    hibernated = body.get("spec", {}).get("jupyterServer", {}).get("hibernated")

    # NOTE: Don't do anything if ``hibernated`` field isn't set
    if hibernated is None:
        return

    if hibernated:
        action = "Hibernating"
        replicas = 0
    else:
        action = "Resuming hibernated"
        replicas = 1

    message = f"{action} Jupyter server {name} in namespace {namespace}"

    try:
        statefulset_api = kubernetes.client.AppsV1Api(kubernetes.client.ApiClient())
        statefulset_api.patch_namespaced_stateful_set(
            name=name,
            namespace=namespace,
            body={
                "spec": {
                    "replicas": replicas,
                },
            },
        )
    except NotFoundError:
        logger.warning(f"{message} failed because we cannot find it")
    except kubernetes.client.ApiException as e:
        logger.error(f"{message} failed with error {e}")
    else:
        logger.info(message)


@kopf.timer(
    config.api_group,
    config.api_version,
    config.custom_resource_name,
    interval=config.JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS,
)
def cull_idle_jupyter_servers(body, name, namespace, logger, **_):
    """
    Check if a session is idle (has zero open connections in proxy and CPU is below
    threshold). If the session is idle then update the jupyter server status with
    the idle duration. If any sessions have been idle for long enough, then cull them.
    """

    def update_last_activity_date(date_str: str):
        if not patch_jupyter_servers(
            name=name,
            namespace=namespace,
            body={
                "metadata": {
                    "annotations": {
                        "renku.io/last-activity-date": date_str,
                    },
                },
            },
        ):
            action = "reset" if date_str == "" else "update"
            logger.warning(
                f"Trying to {action} last activity date for Jupyter server {name} in namespace "
                f"{namespace}, but we cannot find it. Has it been deleted in the meantime?"
            )

    hibernated = body.get("spec", {}).get("jupyterServer", {}).get("hibernated")

    # NOTE: Do nothing if the server is hibernated
    if hibernated:
        last_activity_date = body.get("metadata", {}).get("annotations", {}).get(
            "renku.io/last-activity-date"
        )
        if last_activity_date:
            update_last_activity_date("")
        return

    js_server_status = get_js_server_status(body)
    if js_server_status is None or not isinstance(js_server_status, dict):
        return  # this means server is not fully up and running yet

    idle_seconds_threshold = body["spec"]["culling"]["idleSecondsThreshold"]
    max_age_seconds_threshold = body["spec"]["culling"].get("maxAgeSecondsThreshold", 0)

    try:
        pod_name = body["status"]["mainPod"]["name"]
    except KeyError:
        return
    cpu_usage = get_cpu_usage_for_culling(pod=pod_name, namespace=namespace)

    now = pytz.UTC.localize(datetime.utcnow())
    last_activity = js_server_status.get("last_activity", now)
    jupyter_server_started = js_server_status.get("started", now)
    jupyter_server_age_seconds = (now - jupyter_server_started).total_seconds()
    idle_seconds = (now - last_activity).total_seconds()
    logger.info(
        f"Checking idle status of session {name}, "
        f"idle seconds: {idle_seconds}, "
        f"cpu usage: {cpu_usage}m, "
        f"server status: {js_server_status}, "
        f"age: {jupyter_server_age_seconds} seconds"
    )
    jupyter_server_is_idle_now = (
        cpu_usage <= config.CPU_USAGE_MILLICORES_IDLE_THRESHOLD
        and js_server_status.get("connections", 0) <= 0
        and idle_seconds > config.JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS
    )
    hibernate_idle_server = (
        jupyter_server_is_idle_now and idle_seconds >= idle_seconds_threshold > 0
    )
    hibernate_old_server = jupyter_server_age_seconds >= max_age_seconds_threshold > 0

    if hibernate_idle_server or hibernate_old_server:
        culling_reason = "inactivity" if hibernate_idle_server else "age"
        logger.info(f"Hibernating Jupyter server {name} due to {culling_reason}")

        # NOTE: We don't fill out repository status because we don't want to access session's
        # sidecar in Amalthea
        now = datetime.now(timezone.utc).isoformat(timespec="seconds")
        hibernation = {"branch": "", "commit": "", "dirty": "", "synchronized": "", "date": now}
        if not patch_jupyter_servers(
            name=name,
            namespace=namespace,
            body={
                "metadata": {
                    "annotations": {
                        "renku.io/hibernation": json.dumps(hibernation),
                        "renku.io/hibernation-branch": "",
                        "renku.io/hibernation-commit-sha": "",
                        "renku.io/hibernation-dirty": "",
                        "renku.io/hibernation-synchronized": "",
                        "renku.io/hibernation-date": now,
                        "renku.io/last-activity-date": "",
                    },
                },
                "spec": {
                    "jupyterServer": {
                        "hibernated": True,
                    },
                },
            },
        ):
            logger.warning(
                f"Trying to hibernate idle timer for Jupyter server {name} in namespace "
                f"{namespace}, but we cannot find it. Has it been deleted in the meantime?"
            )
        return

    if jupyter_server_is_idle_now:
        logger.info(
            f"Jupyter Server {name} in namespace {namespace} found to be idle for {idle_seconds}"
        )
        update_last_activity_date(last_activity.isoformat(timespec="seconds"))
    elif idle_seconds > 0:
        logger.info(
            f"Resetting last activity date for Jupyter server {name} in namespace {namespace}."
        )
        update_last_activity_date("")


@kopf.timer(
    config.api_group,
    config.api_version,
    config.custom_resource_name,
    interval=config.JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS,
)
def cull_hibernated_jupyter_servers(body, name, namespace, logger, **_):
    """Check if a server is hibernated for long enough, then cull it."""
    hibernated = body.get("spec", {}).get("jupyterServer", {}).get("hibernated")

    # NOTE: Do nothing if the server isn't hibernated
    if not hibernated:
        return

    hibernated_seconds_threshold = body["spec"]["culling"].get("hibernatedSecondsThreshold", 0)
    if hibernated_seconds_threshold == 0:
        return

    annotations = body.get("metadata", {}).get("annotations", {})
    # NOTE: ``hibernation-date`` is ``""`` when session isn't hibernated; it might not be set when
    # hibernation is enabled by an admin.
    hibernation_date_str = annotations.get("renku.io/hibernation-date", "")

    now = datetime.now(timezone.utc)
    hibernation_date = (
        datetime.fromisoformat(hibernation_date_str)
        if hibernation_date_str
        else now
    )

    hibernated_seconds = (now - hibernation_date).total_seconds()
    can_be_deleted = hibernated_seconds >= hibernated_seconds_threshold

    if can_be_deleted:
        logger.info(f"Deleting hibernated Jupyter server {name} due to age")
        custom_resource_api = get_api(
            config.api_version, config.custom_resource_name, config.api_group
        )
        try:
            custom_resource_api.delete(
                name=name,
                namespace=namespace,
                body=V1DeleteOptions(propagation_policy="Foreground"),
            )
        except NotFoundError:
            logger.warning(
                f"Trying to delete hibernated Jupyter server {name} in namespace {namespace}, "
                "but we cannot find it. Has it been deleted in the meantime?"
            )
    else:
        logger.info(
            f"Jupyter Server {name} in namespace {namespace} found to be hibernated for "
            f"{hibernated_seconds}"
        )
        if not hibernation_date_str:
            if not patch_jupyter_servers(
                name=name,
                namespace=namespace,
                body={
                    "metadata": {
                        "annotations": {
                            "renku.io/hibernation-date": now,
                        },
                    },
                },
            ):
                logger.warning(
                    f"Trying to set hibernation date for Jupyter server {name} in namespace "
                    f"{namespace}, but we cannot find it. Has it been deleted in the meantime?"
                )


@kopf.timer(
    config.api_group,
    config.api_version,
    config.custom_resource_name,
    interval=config.JUPYTER_SERVER_PENDING_CHECK_INTERVAL_SECONDS,
)
def cull_pending_jupyter_servers(body, name, namespace, logger, **kwargs):
    """
    Check if a session is pending (starting or failed). If the session is pending then
    update the jupyter server status with the pending/failed duration. If any sessions
    have been pending for long enough, then cull them.
    """
    starting_seconds_threshold = body["spec"]["culling"]["startingSecondsThreshold"]
    failed_seconds_threshold = body["spec"]["culling"]["failedSecondsThreshold"]
    now = pytz.UTC.localize(datetime.utcnow())
    starting_since = body["status"].get("startingSince")
    failed_since = body["status"].get("failedSince")

    starting_seconds = 0
    failed_seconds = 0

    if starting_since is not None:
        starting_seconds = (now - datetime.fromisoformat(starting_since)).total_seconds()
    if failed_since is not None:
        failed_seconds = (now - datetime.fromisoformat(failed_since)).total_seconds()

    custom_resource_api = get_api(
        config.api_version, config.custom_resource_name, config.api_group
    )

    if starting_seconds_threshold > 0 and starting_seconds > starting_seconds_threshold:
        logger.info(f"Deleting Jupyter server {name} due to starting too long")
        try:
            custom_resource_api.delete(
                name=name,
                namespace=namespace,
                body=V1DeleteOptions(propagation_policy="Foreground"),
            )
        except NotFoundError:
            logger.warning(
                f"Trying to delete Jupyter server {name} in namespace {namespace}, "
                "but we cannot find it. Has it been deleted in the meantime?"
            )
            pass
        return
    if failed_seconds_threshold > 0 and failed_seconds > failed_seconds_threshold:
        logger.info(f"Deleting Jupyter server {name} due to being failed too long")
        try:
            custom_resource_api.delete(
                name=name,
                namespace=namespace,
                body=V1DeleteOptions(propagation_policy="Foreground"),
            )
        except NotFoundError:
            logger.warning(
                f"Trying to delete Jupyter server {name} in namespace {namespace}, "
                "but we cannot find it. Has it been deleted in the meantime?"
            )
            pass
        return


# create @kopf.on.event(...) type of decorators
# Go to the bottom of the update_status function definition to see how
# those decorators are applied.
def get_update_decorator(child_resource_kind):
    return kopf.on.event(
        child_resource_kind["name"],
        group=child_resource_kind["group"],
        labels={config.PARENT_NAME_LABEL_KEY: kopf.PRESENT},
    )


def update_status(body, event, labels, logger, meta, name, namespace, uid, **_):
    """
    Update the custom object status with the status of all children
    and the statefulsets pod as only grand child.
    """

    logger.info(f"{body['kind']}: {event['type']}")

    # Collect labels and other metainformation from the resource which
    # triggered the event.
    parent_name = labels[config.PARENT_NAME_LABEL_KEY]
    parent_uid = labels[config.PARENT_UID_LABEL_KEY]
    child_key = labels.get(config.CHILD_KEY_LABEL_KEY, None)
    owner_references = meta.get("ownerReferences", [])
    owner_uids = [ref["uid"] for ref in owner_references]
    is_main_pod = labels.get(config.MAIN_POD_LABEL_KEY, "") == "true"

    # Check if the jupyter server is the actual parent (ie owner) in order
    # to exclude the grand children of the jupyter server. The only grand child
    # resource we're handling here is the statefulset pod.
    if (parent_uid not in owner_uids) and not is_main_pod:
        logger.info(
            f"Ignoring event for non-child resource of \
            kind {event['type']} on resource of {body['kind']}"
        )
        return

    # Assemble the jsonpatch to update the custom objects status
    try:
        op = config.JSONPATCH_OPS[event["type"]]
    except KeyError:
        # Note: Many events (for example on an initial listing) come without
        # a type. In this case we use "replace" to recover which will also
        # work for not yet existing objects.
        op = "replace"

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
    custom_resource_api = get_api(
        config.api_version, config.custom_resource_name, config.api_group
    )
    try:
        custom_resource_api.patch(
            namespace=namespace,
            name=parent_name,
            body=[patch_op],
            content_type=CONTENT_TYPES["json-patch"],
        )
    # Handle the case when the custom resource is already gone, can
    # happen for removals of children, not for "add" events.
    except NotFoundError as e:
        if op != "add":
            pass
        else:
            raise e


# Add the actual decorators
for child_resource_kind in config.CHILD_RESOURCES:
    get_update_decorator(child_resource_kind)(update_status)


@kopf.on.event("events", field="involvedObject.kind", value="StatefulSet")
def handle_statefulset_events(event, namespace, logger, **_):
    """Used to update the jupyterserver status when a resource quota is full
    and the pod cannot be scheduled."""
    custom_resource_api = get_api(
        config.api_version, config.custom_resource_name, config.api_group
    )
    ss_api = get_api("apps/v1", "StatefulSet")
    body = event.get("object", {})
    tp = body.get("type")
    ss_belongs_to_amalthea = False
    ss = None
    if tp in ["Warning", "Normal"]:
        name = body.get("involvedObject", {}).get("name")
        if name is None:
            return
        try:
            ss = ss_api.get(name=name, namespace=namespace)
        except NotFoundError:
            return
        if ss is None:
            return
        ss_belongs_to_amalthea = (
            ss.metadata.get("labels", {}).get(config.PARENT_NAME_LABEL_KEY) is not None
        )
    if not ss_belongs_to_amalthea:
        return

    patch = None

    try:
        js = custom_resource_api.get(name=name, namespace=namespace)
    except NotFoundError:
        return
    new_message_timestamp = datetime.fromisoformat(body["lastTimestamp"].rstrip("Z"))
    old_message = js.get("status", {}).get("events", {}).get("statefulset", {}).get("message")
    old_message_timestamp_raw = (
        js.get("status", {}).get("events", {}).get("statefulset", {}).get("timestamp")
    )
    old_message_timestamp = None
    if old_message_timestamp_raw is not None:
        old_message_timestamp = datetime.fromisoformat(old_message_timestamp_raw.rstrip("Z"))
    is_event_newer = (
        old_message_timestamp_raw is None
        or (
            new_message_timestamp is not None
            and old_message_timestamp_raw is not None
            and old_message_timestamp is not None
            and new_message_timestamp > old_message_timestamp
        )
    )
    if not is_event_newer:
        return

    if body.get("reason") == "FailedCreate":
        # Add the failing status because the quota is exceeded
        raw_message = body.get("message")
        if raw_message is not None and "exceeded quota" in str(raw_message).lower():
            patch = {
                "status": {
                    "events": {
                        "statefulset": {
                            "message": config.QUOTA_EXCEEDED_MESSAGE,
                            "timestamp": body["lastTimestamp"],
                        }
                    },
                }
            }
    elif body.get("reason") == "SuccessfulCreate":
        # Remove the failing status if it exists because the statefulset was scheduled
        if old_message == config.QUOTA_EXCEEDED_MESSAGE:
            patch = {
                "status": {
                    "events": {
                        "statefulset": {
                            "message": None,
                            "timestamp": body["lastTimestamp"],
                        }
                    },
                }
            }

    if patch is None:
        return

    try:
        custom_resource_api.patch(
            namespace=namespace,
            name=ss.metadata["name"],
            body=patch,
            content_type=CONTENT_TYPES["merge-patch"],
        )
    # Handle the case when the custom resource is already gone
    except NotFoundError:
        pass


def update_resource_usage(body, name, namespace, **kwargs):
    """
    Periodically check the resource usage of the server pod and update the status of the
    JupyterServer resource. Assumes that the relevant container is called jupyter-server
    and that the volume mount in the pod manifest is called workspace.
    """
    try:
        pod_name = body["status"]["mainPod"]["name"]
    except KeyError:
        return

    disk_capacity = get_volume_disk_capacity(pod_name, namespace, "workspace")
    pod_metrics = get_pod_metrics(pod_name, namespace)
    parsed_pod_metrics = parse_pod_metrics(pod_metrics)
    cpu_memory = list(
        filter(lambda x: x.get("name") == "jupyter-server", parsed_pod_metrics)
    )
    cpu_memory = cpu_memory[0] if len(cpu_memory) == 1 else {}
    patch = {
        "status": {
            "mainPod": {
                "resourceUsage": {
                    "disk": {
                        "usedBytes": disk_capacity.get("used_bytes"),
                        "availableBytes": disk_capacity.get("available_bytes"),
                        "totalBytes": disk_capacity.get("total_bytes"),
                    },
                    "cpuMillicores": cpu_memory.get("cpu_millicores"),
                    "memoryBytes": cpu_memory.get("memory_bytes"),
                }
            }
        }
    }
    custom_resource_api = get_api(
        config.api_version, config.custom_resource_name, config.api_group
    )
    try:
        custom_resource_api.patch(
            namespace=namespace,
            name=name,
            body=patch,
            content_type=CONTENT_TYPES["merge-patch"],
        )
    # Handle the case when the custom resource is already gone
    except NotFoundError:
        pass


def publish_metrics(old, new, body, name, **_):
    """Handler to publish prometheus and auditlog metrics on server status change."""
    if old == new:
        # INFO: This is highly unlikely to occur, but if for some reason the status hasn't changed
        # then we do not want to publish a metric. Metrics are published only on status changes.
        return
    metric_event = MetricEvent(
        pytz.UTC.localize(datetime.utcnow()),
        body,
        old_status=old,
        status=new,
    )
    logging.info(
        f"Adding event {metric_event} for server {name} to metrics queue from delete handler."
    )
    metric_events_queue.add_to_queue(metric_event)


if config.JUPYTER_SERVER_RESOURCE_CHECK_ENABLED:
    kopf.timer(
        config.api_group,
        config.api_version,
        config.custom_resource_name,
        interval=config.JUPYTER_SERVER_RESOURCE_CHECK_INTERVAL_SECONDS,
    )(update_resource_usage)


# INFO: Register the functions that publish metric events into the metric events queue
if config.METRICS.enabled or config.AUDITLOG.enabled:
    kopf.on.field(
        config.api_group,
        config.api_version,
        config.custom_resource_name,
        field='status.state',
    )(publish_metrics)
    # NOTE: The 'on.field' handler cannot catch the server deletion so this is needed.
    kopf.on.delete(
        config.api_group,
        config.api_version,
        config.custom_resource_name,
        field='status.state',
    )(publish_metrics)
# INFO: Start the prometheus metrics server if enabled
if config.METRICS.enabled:
    start_http_server(config.METRICS.port)
# INFO: Start a thread to watch the metric events queue and process metrics if handlers are present
if len(metric_handlers) > 0:
    metric_events_queue.start_workers()
