from typing import Any, Dict, List, Union
from prometheus_client import Counter, Histogram, Summary, Gauge
from dateutil import parser
import re

from controller.server_status_enum import ServerStatusEnum
from controller.metrics.events import MetricEventHandler, MetricEvent
from controller.metrics.utils import resource_request_from_manifest


class PrometheusMetric():
    _types = {
        "counter": {
            "usage_methods": ["inc"],
            "type": Counter,
        },
        "gauge": {
            "usage_methods": ["inc", "dec", "set"],
            "type": Gauge,
        },
        "histogram": {
            "usage_methods": ["observe"],
            "type": Histogram,
        },
        "summary": {
            "usage_methods": ["observe"],
            "type": Summary,
        },
    }

    def __init__(
        self,
        metric_type: str,
        name: str,
        documentation: str,
        labelnames: List[str] = [],
        *args,
        **kwargs
    ):
        if metric_type not in self._types:
            raise ValueError(
                f"Metric type {metric_type} not found in allowed types: {self._types.keys()}."
            )
        self.metric_type = metric_type
        self.usage_methods = self._types[metric_type]["usage_methods"]
        sanitized_label_names = [self.sanitize_label_name(label) for label in labelnames]
        self._metric = self._types[metric_type]["type"](
            name, documentation, sanitized_label_names, *args, **kwargs
        )

    @staticmethod
    def sanitize_label_name(val: str) -> str:
        invalid_first_letter = re.compile(r"^[^a-zA-Z_]")
        invalid_all_letters = re.compile(r"[^a-zA-Z0-9_]")
        val = re.sub(invalid_first_letter, "_", val, count=1)
        val = re.sub(invalid_all_letters, "_", val)
        return val

    @classmethod
    def _sanitize_labels(cls, labels: Dict[str, str]) -> Dict[str, str]:
        sanitized_labels = {}
        for name, val in labels.items():
            sanitized_labels[cls.sanitize_label_name(name)] = val
        return sanitized_labels

    def manipulate(
        self,
        operation: str,
        value: Union[int, float],
        labels: Dict[str, str] = {},
    ):
        if operation not in self.usage_methods:
            raise ValueError(
                f"Operation {operation} is not allowed on metric {self.metric_type}. "
                f"Allowed operations are {self.usage_methods}."
            )
        sanitized_labels = self._sanitize_labels(labels)
        operation_method = getattr(
            self._metric if len(sanitized_labels) == 0 else self._metric.labels(sanitized_labels),
            operation,
            None,
        )
        if not operation_method:
            raise AttributeError(
                f"Operation {operation} cannot be found on metric type {self.metric_type}"
            )
        operation_method(value)


class PrometheusMetricHandler(MetricEventHandler):
    def __init__(self, manifest_labelnames: List[str] = []):
        self.manifest_labelnames = manifest_labelnames
        self._sessions_total_created = PrometheusMetric(
            "counter",
            "sessions_total_created",
            "Number of sessions created",
            self.manifest_labelnames,
        )
        self._sessions_total_deleted = PrometheusMetric(
            "counter",
            "sessions_total_deleted",
            "Number of sessions deleted",
            self.manifest_labelnames,
        )
        self._sessions_status_changes = PrometheusMetric(
            "counter",
            "sessions_status_changes",
            "Number of times a status change has occured",
            [
                *self.manifest_labelnames,
                "status_from",
                "status_to",
            ]
        )
        self._sessions_launch_duration = PrometheusMetric(
            "histogram",
            "sessions_launch_duration_seconds",
            "How long did it take for a session to transition into running state",
            self.manifest_labelnames,
            unit="seconds",
        )
        self._sessions_cpu_request = PrometheusMetric(
            "histogram",
            "sessions_cpu_request_millicores",
            "CPU millicores requested by a user for a session.",
            self.manifest_labelnames,
            unit="m",
        )
        self._sessions_memory_request = PrometheusMetric(
            "histogram",
            "sessions_memory_request_bytes",
            "Memory requested by a user for a session.",
            self.manifest_labelnames,
            unit="byte",
        )
        self._sessions_gpu_request = PrometheusMetric(
            "histogram",
            "sessions_gpu_request",
            "GPUs requested by a user for a session.",
            self.manifest_labelnames,
        )
        self._sessions_disk_request = PrometheusMetric(
            "histogram",
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
        self._sessions_total_created.manipulate("inc", 1, manifest_labels)
        resource_request = resource_request_from_manifest(metric_event.session)
        if not resource_request:
            return
        self._sessions_cpu_request.manipulate(
            "observe",
            resource_request.cpu_millicores,
            manifest_labels,
        )
        self._sessions_disk_request.manipulate(
            "observe",
            resource_request.disk_bytes,
            manifest_labels,
        )
        self._sessions_memory_request.manipulate(
            "observe",
            resource_request.memory_bytes,
            manifest_labels,
        )
        self._sessions_gpu_request.manipulate(
            "observe",
            resource_request.gpus,
            manifest_labels,
        )

    def _on_stop(self, metric_event: MetricEvent):
        manifest_labels = self._collect_labels_from_manifest(metric_event.session)
        self._sessions_total_deleted.manipulate("inc", 1, manifest_labels)

    def _on_any_status_change(self, metric_event: MetricEvent):
        manifest_labels = self._collect_labels_from_manifest(metric_event.session)
        old_status = metric_event.old_status
        new_status = metric_event.session.get("status", {}).get("state")
        if new_status:
            new_status = ServerStatusEnum(new_status)
        if (
            old_status == ServerStatusEnum.Starting
            and new_status == ServerStatusEnum.Running
        ):
            started_at_str = metric_event.session.get("metadata", {}).get("creationTimestamp")
            started_at = parser.isoparse(started_at_str) if started_at_str else None
            if started_at:
                launch_duration = (metric_event.event_timestamp - started_at).total_seconds()
                self._sessions_launch_duration.manipulate(
                    "observe",
                    launch_duration,
                    manifest_labels,
                )
        if old_status != new_status:
            status_change_labels = {
                **manifest_labels,
                "status_from": old_status.value if old_status else "None",
                "status_to": new_status.value if new_status else "None",
            }
            self._sessions_status_changes.manipulate("inc", 1, status_change_labels)

    def publish(self, metric_event: MetricEvent):
        old_status = metric_event.old_status
        new_status = metric_event.session.get("status", {}).get("state")
        if new_status:
            new_status = ServerStatusEnum(new_status)
        if new_status == old_status:
            return
        if old_status is None and new_status == ServerStatusEnum.Starting:
            self._on_start(metric_event)
        if new_status == ServerStatusEnum.Stopping:
            self._on_stop(metric_event)
        self._on_any_status_change(metric_event)
