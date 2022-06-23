from abc import ABC, abstractmethod
from dataclasses import dataclass, asdict
from typing import Optional
from dateutil import parser
from datetime import datetime, timedelta
import boto3
from smart_open import open as sopen
import pytz
from uuid import uuid4
from pathlib import Path
import json
import dataconf

from controller.server_status_enum import ServerStatusEnum
from controller.metrics.events import MetricEventHandler, MetricEvent
from controller.metrics.utils import ResourceRequest, resource_request_from_manifest


@dataclass
class S3Config:
    endpoint: str
    bucket: str
    path_prefix: str
    access_key_id: str
    secret_access_key: str

    @classmethod
    def dataconf_from_env(cls, prefix="AUDITLOG_S3_"):
        return dataconf.env(prefix, cls)


@dataclass
class SesionMetricData:
    name: str
    namespace: str
    uid: str
    creation_timestamp: datetime
    resource_request: Optional[ResourceRequest]
    image: str
    status: Optional[ServerStatusEnum]
    old_status: Optional[ServerStatusEnum]
    commit: Optional[str]
    repository_url: Optional[str]
    user: Optional[str]

    def _default_json_serializer(obj):
        if type(obj) is datetime:
            return obj.isoformat()
        if type(obj) is ServerStatusEnum:
            return obj.value

    def to_json(self):
        return json.dumps(asdict(self), default=self._default_json_serializer, indent=None)

    @classmethod
    def from_metric_event(cls, metric_event: MetricEvent):
        manifest = metric_event.session
        return cls(
            manifest.get("metadata", {}).get("name"),
            manifest.get("metadata", {}).get("namespace"),
            manifest.get("metadata", {}).get("uid"),
            parser.isoparse(manifest.get("metadata", {}).get("creationTimestamp")),
            resource_request_from_manifest(manifest),
            manifest.get("spec", {}).get("jupyterServer", {}).get("image"),
            (
                ServerStatusEnum(manifest.get("status", {}).get("state"))
                if manifest.get("status", {}).get("state")
                else None
            ),
            metric_event.old_status,
            manifest.get("metadata", {}).get("annotations", {}).get("renku.io/commit-sha"),
            manifest.get("metadata", {}).get("annotations", {}).get("renku.io/repository"),
            manifest.get("metadata", {}).get("annotations", {}).get("renku.io/username"),
        )


class BaseMetricLogger(ABC):
    @abstractmethod
    def log(val: str):
        pass


class RotatingS3Log(BaseMetricLogger):
    _datetime_format = "%Y%m%d_%H%M%S%z"

    def __init__(self, config: S3Config, period_hours: 24):
        self.config = config
        self.period_hours = period_hours
        self._session = boto3.Session(
            aws_secret_access_key=config.secret_access_key, 
            aws_access_key_id=config.access_key_id,
        )
        self._client = self._session.client(
            "s3",
            endpoint_url=config.endpoint,
        )
        # INFO: Ensure that bucket exists by calling head_bucket
        self._client.head_bucket(Bucket=config.bucket)
        self._current_log_start_timestamp = None
        self._current_log_id = uuid4()
        self.__file_object = None

    @property
    def _file_object(self):
        if not self._current_log_start_timestamp:
            self._current_log_start_timestamp = pytz.UTC.localize(datetime.utcnow())
        now = pytz.UTC.localize(datetime.utcnow())
        if (now - self._current_log_start_timestamp).total_seconds() / 3600 > self.period_hours:
            # INFO: It is time to rotate logs
            self._current_log_start_timestamp += timedelta(hours=self.period_hours)
            if self.__file_object:
                self.__file_object.close()
            self.__file_object = self._open_s3_file()
            return self.__file_object
        if not self.__file_object:
            # INFO: S3 file is not open at all
            self.__file_object = self._open_s3_file()
            return self.__file_object

    def _open_s3_file(self):
        uri = "s3://{}/{}".format(
            self.config.bucket,
            str(
                Path(self.config.path_prefix) / "{}_{}.txt".format(
                    self._current_log_start_timestamp.strftime(self._datetime_format),
                    self._current_log_id,
                )
            ),
        )
        return sopen(
            uri, mode="w", buffering=0, transport_params={'client': self._client}
        )

    def log(self, val: str):
        self._file_object.write(val)


class S3MetricHandler(MetricEventHandler):
    def __init__(self, logger: BaseMetricLogger):
        self.logger = logger

    def publish(self, metric_event: MetricEvent):
        session_metric_data = SesionMetricData.from_metric_event(metric_event)
        self.logger.log(session_metric_data.to_json())
        self.logger.log("\n")
