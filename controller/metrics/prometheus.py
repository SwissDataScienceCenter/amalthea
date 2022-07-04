from typing import Any, Dict, List, Union, NamedTuple, Optional
from prometheus_client import Counter, Histogram, Summary, Gauge
from prometheus_client.metrics import MetricWrapperBase
import re
from enum import Enum

from controller.server_status_enum import ServerStatusEnum
from controller.metrics.events import MetricEventHandler, MetricEvent
from controller.metrics.utils import resource_request_from_manifest, additional_labels_from_manifest


class PrometheusMetricAction(Enum):
    """The different methods that can be used to manipulate prometheus metrics."""
    inc = "inc"
    dec = "dec"
    set = "set"
    observe = "observe"


class PrometheusMetricType(NamedTuple):
    """A generic prometheus metric "struct" with its allowed
    methods and the specific metric type."""
    type: MetricWrapperBase
    actions: List[PrometheusMetricAction]


class PrometheusMetricTypesEnum(Enum):
    """All the different prometheus metrics supported."""
    counter = PrometheusMetricType(Counter, [PrometheusMetricAction.inc])
    gauge = PrometheusMetricType(
        Gauge,
        [PrometheusMetricAction.inc, PrometheusMetricAction.dec, PrometheusMetricAction.set],
    )
    histogram = PrometheusMetricType(Histogram, [PrometheusMetricAction.observe])
    summary = PrometheusMetricType(Summary, [PrometheusMetricAction.observe])


class PrometheusMetric():
    """A generic wrapper class for all prometheus metrics."""
    _label_name_invalid_first_letter = re.compile(r"^[^a-zA-Z_]")
    _label_name_invalid_all_letters = re.compile(r"[^a-zA-Z0-9_]")

    def __init__(
        self,
        metric_type: Union[str, PrometheusMetricType],
        name: str,
        documentation: str,
        labelnames: Optional[List[str]],
        *args,
        **kwargs
    ):
        if type(metric_type) is str:
            self._metric_type = PrometheusMetricTypesEnum[metric_type].value
        elif type(metric_type) is PrometheusMetricType:
            self._metric_type = metric_type
        else:
            raise ValueError(
                "Invalid type provided for metric_type, needed str or "
                f"PrometheusMetricType, got {type(metric_type)}"
            )
        if labelnames is None:
            labelnames = []
        sanitized_label_names = [self.sanitize_label_name(label) for label in labelnames]
        self._metric = self._metric_type.type(
            name, documentation, sanitized_label_names, *args, **kwargs
        )

    def sanitize_label_name(self, val: str) -> str:
        """Certain characters are not allowed in prometheus metric labels.
        This method removes those values and replaces them with underscores."""
        val = re.sub(self._label_name_invalid_first_letter, "_", val, count=1)
        val = re.sub(self._label_name_invalid_all_letters, "_", val)
        return val

    def _sanitize_labels(self, labels: Dict[str, str]) -> Dict[str, str]:
        return {
            self.sanitize_label_name(name): val for name, val in labels.items()
        }

    def __call__(
        self,
        action: Union[str, PrometheusMetricAction],
        value: Union[int, float],
        labels: Dict[str, str] = None,
    ):
        """Manipulates the actual prometheus metric so that
        the metric is actually published and can be scraped by prometheus or similar
        applications.
        """
        if type(action) is str:
            metric_action = PrometheusMetricAction(action)
        elif type(action) is PrometheusMetricAction:
            metric_action = action
        else:
            raise ValueError(
                "Type of metric action needs to be either str or "
                f"PrometheusMetricAction, got {type(action)}"
            )
        if metric_action not in self._metric_type.actions:
            raise ValueError(
                f"Action {metric_action} is not allowed on metric {self._metric_type}. "
                f"Allowed operations are {self._metric_type.actions}."
            )
        if labels:
            sanitized_labels = self._sanitize_labels(labels)
        else:
            sanitized_labels = {}
        operation_method = getattr(
            self._metric if len(sanitized_labels) == 0 else self._metric.labels(**sanitized_labels),
            action.value,
            None,
        )
        operation_method(value)


class PrometheusMetricNames(Enum):
    """Used to avoid errors in metric names and to ensure
    that always the same set of metric names are used."""
    sessions_total_created = "sessions_total_created"
    sessions_total_deleted = "sessions_total_deleted"
    sessions_status_changes = "sessions_status_changes"
    sessions_launch_duration_seconds = "sessions_launch_duration_seconds"
    sessions_cpu_request_millicores = "sessions_cpu_request_millicores"
    sessions_memory_request_bytes = "sessions_memory_request_bytes"
    sessions_gpu_request = "sessions_gpu_request"
    sessions_disk_request_bytes = "sessions_disk_request_bytes"


class PrometheusMetricHandler(MetricEventHandler):
    """Handles metric events from the queue that are created by amalthea."""
    def __init__(self, manifest_labelnames: List[str] = []):
        self.manifest_labelnames = manifest_labelnames
        self._sessions_total_created = PrometheusMetric(
            PrometheusMetricTypesEnum["counter"].value,
            PrometheusMetricNames["sessions_total_created"].value,
            "Number of sessions created",
            self.manifest_labelnames,
        )
        self._sessions_total_deleted = PrometheusMetric(
            PrometheusMetricTypesEnum["counter"].value,
            PrometheusMetricNames["sessions_total_deleted"].value,
            "Number of sessions deleted",
            self.manifest_labelnames,
        )
        self._sessions_status_changes = PrometheusMetric(
            PrometheusMetricTypesEnum["counter"].value,
            PrometheusMetricNames["sessions_status_changes"].value,
            "Number of times a status change has occured",
            [
                *self.manifest_labelnames,
                "status_from",
                "status_to",
            ]
        )
        self._sessions_launch_duration = PrometheusMetric(
            PrometheusMetricTypesEnum["histogram"].value,
            PrometheusMetricNames["sessions_launch_duration_seconds"].value,
            "How long did it take for a session to transition into running state",
            self.manifest_labelnames,
            unit="seconds",
        )
        self._sessions_cpu_request = PrometheusMetric(
            PrometheusMetricTypesEnum["histogram"].value,
            PrometheusMetricNames["sessions_cpu_request_millicores"].value,
            "CPU millicores requested by a user for a session.",
            self.manifest_labelnames,
            unit="m",
        )
        self._sessions_memory_request = PrometheusMetric(
            PrometheusMetricTypesEnum["histogram"].value,
            PrometheusMetricNames["sessions_memory_request_bytes"].value,
            "Memory requested by a user for a session.",
            self.manifest_labelnames,
            unit="byte",
        )
        self._sessions_gpu_request = PrometheusMetric(
            PrometheusMetricTypesEnum["histogram"].value,
            PrometheusMetricNames["sessions_gpu_request"].value,
            "GPUs requested by a user for a session.",
            self.manifest_labelnames,
        )
        self._sessions_disk_request = PrometheusMetric(
            PrometheusMetricTypesEnum["histogram"].value,
            PrometheusMetricNames["sessions_disk_request_bytes"].value,
            "Disk space requested by a user for a session.",
            self.manifest_labelnames,
            unit="byte",
        )

    def _collect_labels_from_manifest(self, manifest: Dict[str, Any]) -> Dict[str, str]:
        return additional_labels_from_manifest(manifest, self.manifest_labelnames)

    def _on_start(self, metric_event: MetricEvent):
        manifest_labels = self._collect_labels_from_manifest(metric_event.session)
        self._sessions_total_created(PrometheusMetricAction.inc, 1, manifest_labels)
        resource_request = resource_request_from_manifest(metric_event.session)
        if not resource_request:
            return
        self._sessions_cpu_request(
            PrometheusMetricAction.observe,
            resource_request.cpu_millicores,
            manifest_labels,
        )
        self._sessions_disk_request(
            PrometheusMetricAction.observe,
            resource_request.disk_bytes,
            manifest_labels,
        )
        self._sessions_memory_request(
            PrometheusMetricAction.observe,
            resource_request.memory_bytes,
            manifest_labels,
        )
        self._sessions_gpu_request(
            PrometheusMetricAction.observe,
            resource_request.gpus,
            manifest_labels,
        )

    def _on_stop(self, metric_event: MetricEvent):
        manifest_labels = self._collect_labels_from_manifest(metric_event.session)
        self._sessions_total_deleted(PrometheusMetricAction.inc, 1, manifest_labels)

    def _on_any_status_change(self, metric_event: MetricEvent):
        manifest_labels = self._collect_labels_from_manifest(metric_event.session)
        if (
            metric_event.old_status == ServerStatusEnum.Starting
            and metric_event.status == ServerStatusEnum.Running
        ):
            if metric_event.sessionCreationTimestamp:
                launch_duration = (
                    metric_event.event_timestamp - metric_event.sessionCreationTimestamp
                ).total_seconds()
                self._sessions_launch_duration(
                    PrometheusMetricAction.observe,
                    launch_duration,
                    manifest_labels,
                )
        if metric_event.status != metric_event.old_status:
            status_change_labels = {
                **manifest_labels,
                "status_from": metric_event.old_status.value if metric_event.old_status else "None",
                "status_to": metric_event.status.value if metric_event.status else "None",
            }
            self._sessions_status_changes(PrometheusMetricAction.inc, 1, status_change_labels)

    def publish(self, metric_event: MetricEvent):
        """Publishes (i.e. persists) the proper prometheus metrics
        depending on the old and new statuses of the jupyterserver."""
        old_status = metric_event.old_status
        new_status = metric_event.status
        if new_status == old_status:
            return
        if old_status is None and new_status == ServerStatusEnum.Starting:
            self._on_start(metric_event)
        if new_status == ServerStatusEnum.Stopping:
            self._on_stop(metric_event)
        self._on_any_status_change(metric_event)
