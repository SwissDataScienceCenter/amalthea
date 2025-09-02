import logging
import threading
from kopf._core.actions import loggers
import uvloop

import kopf
from prometheus_client import start_http_server
from kubernetes import config as k8s_config

from controller import config
from controller.metrics.prometheus import PrometheusMetricHandler
from controller.metrics.queue import MetricsQueue
from controller.metrics.s3 import S3Formatter, S3MetricHandler, S3RotatingLogHandler
from controller.server_controller import (
    configure,
    create_fn,
    cull_hibernated_jupyter_servers,
    cull_idle_jupyter_servers,
    cull_pending_jupyter_servers,
    delete_fn,
    handle_statefulset_events,
    hibernation_field_handler,
    publish_metrics,
    resources_field_handler,
    update_resource_usage,
    update_server_state,
    update_status,
)


def register_jupyter_server_handlers(
    registry: kopf.OperatorRegistry,
    metric_events_queue: MetricsQueue | None = None,
) -> kopf.OperatorRegistry:
    logging.info("Populating regsitry")
    kopf.on.startup(registry=registry)(configure)
    kopf.on.delete(
        config.api_group,
        config.api_version,
        config.custom_resource_name,
        registry=registry,
    )(delete_fn)
    kopf.on.create(
        config.api_group,
        config.api_version,
        config.custom_resource_name,
        timeout=config.KOPF_CREATE_TIMEOUT,
        retries=config.KOPF_CREATE_RETRIES,
        backoff=config.KOPF_CREATE_BACKOFF,
        registry=registry,
    )(create_fn)
    kopf.on.event(
        version=config.api_version,
        kind=config.custom_resource_name,
        group=config.api_group,
        registry=registry,
    )(update_server_state)
    kopf.on.field(
        group=config.api_group,
        version=config.api_version,
        kind=config.custom_resource_name,
        field="spec.jupyterServer.hibernated",
        registry=registry,
    )(hibernation_field_handler)
    kopf.on.field(
        group=config.api_group,
        version=config.api_version,
        kind=config.custom_resource_name,
        field="spec.jupyterServer.resources",
        registry=registry,
    )(resources_field_handler)
    kopf.timer(
        config.api_group,
        config.api_version,
        config.custom_resource_name,
        interval=config.JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS,
        registry=registry,
    )(cull_idle_jupyter_servers)
    kopf.timer(
        config.api_group,
        config.api_version,
        config.custom_resource_name,
        interval=config.JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS,
        registry=registry,
    )(cull_hibernated_jupyter_servers)
    kopf.timer(
        config.api_group,
        config.api_version,
        config.custom_resource_name,
        interval=config.JUPYTER_SERVER_PENDING_CHECK_INTERVAL_SECONDS,
        registry=registry,
    )(cull_pending_jupyter_servers)
    for child_resource_kind in config.CHILD_RESOURCES:
        logging.info(f"Setting up child resource {child_resource_kind}")
        kopf.on.event(
            child_resource_kind["name"],
            group=child_resource_kind["group"],
            labels={config.PARENT_NAME_LABEL_KEY: kopf.PRESENT},
            registry=registry,
        )(update_status)
    kopf.on.event(
        "events",
        field="involvedObject.kind",
        value="StatefulSet",
        registry=registry,
    )(handle_statefulset_events)
    if config.JUPYTER_SERVER_RESOURCE_CHECK_ENABLED:
        kopf.timer(
            config.api_group,
            config.api_version,
            config.custom_resource_name,
            interval=config.JUPYTER_SERVER_RESOURCE_CHECK_INTERVAL_SECONDS,
            registry=registry,
        )(update_resource_usage)
    # INFO: Register the functions that publish metric events into the metric events queue
    if config.METRICS.enabled or config.AUDITLOG.enabled:
        if metric_events_queue is None:
            raise Exception("The metric events queue has to be initialized when metrics are enabled")
        kopf.on.field(
            config.api_group,
            config.api_version,
            config.custom_resource_name,
            field="status.state",
            registry=registry,
        )(publish_metrics(metric_events_queue))
        # NOTE: The 'on.field' handler cannot catch the server deletion so this is needed.
        kopf.on.delete(
            config.api_group,
            config.api_version,
            config.custom_resource_name,
            field="status.state",
            registry=registry,
        )(publish_metrics(metric_events_queue))

    def login_fn(**kwargs):
        sa_login = kopf.login_with_service_account(**kwargs)
        kubeconfig_login = None
        if sa_login is not None:
            k8s_config.load_incluster_config()
            logging.info("Logged in with a K8s service account")
            return sa_login
        kubeconfig_login = kopf.login_with_kubeconfig()
        if kubeconfig_login is None:
            raise Exception("Cannot login with service account or kubeconfig")
        k8s_config.load_kube_config()
        logging.info("Logged in with kubeconfig")
        return kubeconfig_login

    kopf.on.login(registry=registry)(login_fn)

    logging.info("Finished populating regsitry")
    return registry


async def run(ready_flag: threading.Event | None = None, stop_flag: threading.Event | None = None):
    loggers.configure(debug=config.DEBUG, verbose=config.VERBOSE, log_format=loggers.LogFormat.JSON)

    metric_handlers = []
    if config.METRICS.enabled:
        metric_handlers.append(PrometheusMetricHandler(config.METRICS.extra_labels))
    if config.AUDITLOG.enabled:
        s3_metric_logger = logging.getLogger("s3")
        s3_logging_handler = S3RotatingLogHandler("/tmp/amalthea_audit_log.txt", "a", config.AUDITLOG.s3)
        s3_logging_handler.setFormatter(S3Formatter())
        s3_metric_logger.addHandler(s3_logging_handler)
        metric_handlers.append(S3MetricHandler(s3_metric_logger, config.AUDITLOG))
    metric_events_queue = MetricsQueue(metric_handlers)

    registry = kopf.OperatorRegistry()
    registry = register_jupyter_server_handlers(registry, metric_events_queue)

    # INFO: Start the prometheus metrics server if enabled
    if config.METRICS.enabled:
        start_http_server(config.METRICS.port)
    # INFO: Start a thread to watch the metric events queue and process metrics if handlers are present
    if len(metric_handlers) > 0:
        metric_events_queue.start_workers()

    logging.info(
        f"Starting the operator {config.NAMESPACES} {config.api_version} "
        f"{config.api_group} {config.custom_resource_name}"
    )
    logging.info(f"The activity hanlders we start with is {len(registry._activities._handlers)}")
    logging.info(f"The indexing handlers we start with is {len(registry._indexing._handlers)}")
    logging.info(f"The watching handlers we start with is {len(registry._watching._handlers)}")
    logging.info(f"The spawning handlers we start with is {len(registry._spawning._handlers)}")
    logging.info(f"The changing handlers we start with is {len(registry._changing._handlers)}")
    logging.info(f"The webhooks handlers we start with is {len(registry._webhooks._handlers)}")

    await kopf.operator(
        registry=registry,
        clusterwide=config.CLUSTER_WIDE,
        namespaces=config.NAMESPACES,
        ready_flag=ready_flag,
        stop_flag=stop_flag,
        liveness_endpoint="http://0.0.0.0:8080/healthz",
    )


if __name__ == "__main__":
    uvloop.run(run())
