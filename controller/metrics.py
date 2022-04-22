from dataclasses import dataclass
from prometheus_client import Counter, Gauge
from typing import Dict

from controller import config


@dataclass
class Metrics:
    total_launch: Counter = Counter(
        "sessions_launch_total",
        "Number of sessions launched",
        labelnames=config.METRICS_EXTRA_LABELS_SANITIZED
    )
    total_failed: Counter = Counter(
        "sessions_failed_total",
        "Number of sessions which have failed being launched",
        labelnames=config.METRICS_EXTRA_LABELS_SANITIZED
    )
    num_of_sessions: Gauge = Gauge(
        "sessions_all",
        "Number of sessions",
        labelnames=config.METRICS_EXTRA_LABELS_SANITIZED
    )
    num_of_running_sessions: Gauge = Gauge(
        "sessions_running",
        "Number of sessions in running state",
        labelnames=config.METRICS_EXTRA_LABELS_SANITIZED,
    )
    num_of_starting_sessions: Gauge = Gauge(
        "sessions_starting",
        "Number of sessions which have been scheduled but are still starting",
        labelnames=config.METRICS_EXTRA_LABELS_SANITIZED,
    )

    def manipulate(
        self,
        metric_name: str,
        operation: str,
        manifest_labels: Dict[str, str] = {},
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
            metric = metric.labels(**labels)
        getattr(metric, operation)()
