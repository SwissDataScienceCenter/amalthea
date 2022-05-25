from dataclasses import dataclass
from prometheus_client import Counter, Histogram
from typing import Dict, Union

from controller import config


@dataclass
class Metrics:
    sessions_total_created: Counter = Counter(
        "sessions_total_created",
        "Number of sessions created",
        labelnames=config.METRICS_EXTRA_LABELS_SANITIZED
    )
    sessions_total_deleted: Counter = Counter(
        "sessions_total_deleted",
        "Number of sessions deleted",
        labelnames=config.METRICS_EXTRA_LABELS_SANITIZED
    )
    sessions_status_changes: Counter = Counter(
        "sessions_status_changes",
        "Number of times a status change has occured",
        labelnames=[
            *config.METRICS_EXTRA_LABELS_SANITIZED,
            "status_from",
            "status_to",
        ]
    )
    sessions_launch_duration: Histogram = Histogram(
        "sessions_launch_duration_seconds",
        "How long did it take for a session to transition into running state",
        unit="seconds",
        labelnames=config.METRICS_EXTRA_LABELS_SANITIZED
    )
    sessions_cpu_request: Histogram = Histogram(
        "sessions_cpu_request_millicores",
        "CPU millicores requested by a user for a session.",
        unit="m",
        labelnames=config.METRICS_EXTRA_LABELS_SANITIZED
    )
    sessions_memory_request: Histogram = Histogram(
        "sessions_memory_request_bytes",
        "Memory requested by a user for a session.",
        unit="byte",
        labelnames=config.METRICS_EXTRA_LABELS_SANITIZED
    )
    sessions_disk_request: Histogram = Histogram(
        "sessions_disk_request_bytes",
        "Disk space requested by a user for a session.",
        unit="byte",
        labelnames=config.METRICS_EXTRA_LABELS_SANITIZED
    )

    def manipulate(
        self,
        metric_name: str,
        operation: str,
        value: Union[int, float],
        manifest_labels: Dict[str, str] = {},
        extra_labels: Dict[str, str] = {},
    ):
        metric = getattr(self, metric_name)
        if len(config.METRICS_EXTRA_LABELS) > 0:
            labels = {}
            for i in range(len(config.METRICS_EXTRA_LABELS)):
                try:
                    labels[config.METRICS_EXTRA_LABELS_SANITIZED[i]] = manifest_labels[
                        config.METRICS_EXTRA_LABELS[i]
                    ]
                except KeyError:
                    # NOTE: We should not be super sensitive to k8s label changes, so if a
                    # lable that is exepected to exist in the manifest does not then
                    # the metric label will not be applied, but the metric will be counted
                    labels[config.METRICS_EXTRA_LABELS_SANITIZED[i]] = "Unknown"
            metric = metric.labels(**labels, **extra_labels)
        getattr(metric, operation)(value)
