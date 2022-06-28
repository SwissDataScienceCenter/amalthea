from typing import Any, Dict, List, Union, NamedTuple, Optional
from prometheus_client import Counter, Histogram, Summary, Gauge
from prometheus_client.metrics import MetricWrapperBase
import re
from enum import Enum

from controller.server_status_enum import ServerStatusEnum
from controller.metrics.events import MetricEventHandler, MetricEvent
from controller.metrics.utils import resource_request_from_manifest


class PrometheusMetricAction(Enum):
    inc = "inc"
    dec = "dec"
    set = "set"
    observe = "observe"


class PrometheusMetricType(NamedTuple):
    type: MetricWrapperBase
    actions: List[PrometheusMetricAction]


class PrometheusMetricTypesEnum(Enum):
    counter = PrometheusMetricType(Counter, [PrometheusMetricAction.inc])
    gauge = PrometheusMetricType(
        Gauge,
        [PrometheusMetricAction.inc, PrometheusMetricAction.dec, PrometheusMetricAction.set],
    )
    histogram = PrometheusMetricType(Histogram, [PrometheusMetricAction.observe])
    summary = PrometheusMetricType(Summary, [PrometheusMetricAction.observe])


class PrometheusMetric():
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
        labels: Dict[str, str] = {},
    ):
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
        sanitized_labels = self._sanitize_labels(labels)
        operation_method = getattr(
            self._metric if len(sanitized_labels) == 0 else self._metric.labels(**sanitized_labels),
            action,
            None,
        )
        operation_method(value)


class PrometheusMetricHandler(MetricEventHandler):
    def __init__(self, manifest_labelnames: List[str] = []):
        self.manifest_labelnames = manifest_labelnames
        self._sessions_total_created = PrometheusMetric(
            PrometheusMetricTypesEnum["counter"].value,
            "sessions_total_created",
            "Number of sessions created",
            self.manifest_labelnames,
        )
        self._sessions_total_deleted = PrometheusMetric(
            PrometheusMetricTypesEnum["counter"].value,
            "sessions_total_deleted",
            "Number of sessions deleted",
            self.manifest_labelnames,
        )
        self._sessions_status_changes = PrometheusMetric(
            PrometheusMetricTypesEnum["counter"].value,
            "sessions_status_changes",
            "Number of times a status change has occured",
            [
                *self.manifest_labelnames,
                "status_from",
                "status_to",
            ]
        )
        self._sessions_launch_duration = PrometheusMetric(
            PrometheusMetricTypesEnum["histogram"].value,
            "sessions_launch_duration_seconds",
            "How long did it take for a session to transition into running state",
            self.manifest_labelnames,
            unit="seconds",
        )
        self._sessions_cpu_request = PrometheusMetric(
            PrometheusMetricTypesEnum["histogram"].value,
            "sessions_cpu_request_millicores",
            "CPU millicores requested by a user for a session.",
            self.manifest_labelnames,
            unit="m",
        )
        self._sessions_memory_request = PrometheusMetric(
            PrometheusMetricTypesEnum["histogram"].value,
            "sessions_memory_request_bytes",
            "Memory requested by a user for a session.",
            self.manifest_labelnames,
            unit="byte",
        )
        self._sessions_gpu_request = PrometheusMetric(
            PrometheusMetricTypesEnum["histogram"].value,
            "sessions_gpu_request",
            "GPUs requested by a user for a session.",
            self.manifest_labelnames,
        )
        self._sessions_disk_request = PrometheusMetric(
            PrometheusMetricTypesEnum["histogram"].value,
            "sessions_disk_request_bytes",
            "Disk space requested by a user for a session.",
            self.manifest_labelnames,
            unit="byte",
        )

    def _collect_labels_from_manifest(self, manifest: Dict[str, Any]) -> Dict[str, str]:
        metric_labels = {}
        manifest_labels = manifest.get("metadata", {}).get("labels", {})
        if len(self.manifest_labelnames) > 0:
            for label_name in self.manifest_labelnames:
                metric_labels[label_name] = manifest_labels.get(label_name, "Unknown")
        return metric_labels

    def _on_start(self, metric_event: MetricEvent):
        manifest_labels = self._collect_labels_from_manifest(metric_event.session)
        self._sessions_total_created("inc", 1, manifest_labels)
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
            started_at = metric_event.session.get("metadata", {}).get("creationTimestamp")
            if started_at:
                launch_duration = (metric_event.event_timestamp - started_at).total_seconds()
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
        old_status = metric_event.old_status
        new_status = metric_event.session.get("status", {}).get("state")
        if new_status == old_status:
            return
        if old_status is None and new_status == ServerStatusEnum.Starting:
            self._on_start(metric_event)
        if new_status == ServerStatusEnum.Stopping:
            self._on_stop(metric_event)
        self._on_any_status_change(metric_event)
