from dataclasses import dataclass, field
import dataconf
import json
from typing import Optional, Union, List


@dataclass
class S3Config:
    """The configuration needed to upload metrics to S3."""

    endpoint: str
    bucket: str
    path_prefix: str
    access_key_id: str
    secret_access_key: str
    rotation_period_seconds: Union[str, int] = 86400

    def __post_init__(self):
        if type(self.rotation_period_seconds) is str:
            self.rotation_period_seconds = int(self.rotation_period_seconds)


@dataclass
class MetricsBaseConfig:
    """Base metrics/auditlog configuration."""

    enabled: Union[str, bool] = False
    extra_labels: Union[str, List[str]] = field(default_factory=list)

    def __post_init__(self):
        if type(self.enabled) is str:
            self.enabled = self.enabled.lower() == "true"
        if type(self.extra_labels) is str:
            self.extra_labels = json.loads(self.extra_labels)


@dataclass
class AuditlogConfig(MetricsBaseConfig):
    """The configuration used for the auditlogs."""

    s3: Optional[S3Config] = None

    def __post_init__(self):
        super().__post_init__()
        if self.enabled and not self.s3:
            raise ValueError(
                "If auditlog is enabled then the S3 configuration has to be provided."
            )

    @classmethod
    def dataconf_from_env(cls, prefix="AUDITLOG_"):
        return dataconf.env(prefix, cls)


@dataclass
class PrometheusMetricsConfig(MetricsBaseConfig):
    """The configuration for prometheus metrics"""

    port: Union[str, int] = 8765

    def __post_init__(self):
        super().__post_init__()
        if type(self.port) is str:
            self.port = int(self.port)

    @classmethod
    def dataconf_from_env(cls, prefix="METRICS_"):
        return dataconf.env(prefix, cls)
