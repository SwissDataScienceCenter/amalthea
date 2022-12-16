import atexit
import json
import os
import re
from dataclasses import asdict, dataclass, field
from datetime import datetime, timedelta
from logging import Formatter, Logger, LogRecord
from logging.handlers import BaseRotatingHandler
from pathlib import Path
from typing import Dict, List, Optional

import boto3
import pytz

from controller.config_types import AuditlogConfig, S3Config
from controller.metrics.events import MetricEvent, MetricEventHandler
from controller.metrics.utils import (
    ResourceRequest,
    additional_labels_from_manifest,
    resource_request_from_manifest,
)
from controller.server_status_enum import ServerStatusEnum


@dataclass
class SesionMetricData:
    """The data that is included for each metric event uploaded to S3."""

    name: str
    namespace: str
    uid: str
    creation_timestamp: datetime
    resource_request: Optional[ResourceRequest]
    image: str
    status: Optional[ServerStatusEnum]
    old_status: Optional[ServerStatusEnum]
    additional_labels: Optional[Dict[str, str]] = field(default_factory=dict)

    def __post_init__(self):
        if not self.additional_labels:
            self.additional_labels = {}

    @staticmethod
    def _default_json_serializer(obj):
        if type(obj) is datetime:
            return obj.isoformat()
        if type(obj) is ServerStatusEnum:
            return obj.value

    def __str__(self):
        return json.dumps(asdict(self), default=self._default_json_serializer, indent=None)

    @classmethod
    def from_metric_event(
        cls,
        metric_event: MetricEvent,
        additional_label_names: Optional[List[str]] = None,
    ):
        manifest = metric_event.session
        return cls(
            manifest.get("metadata", {}).get("name"),
            manifest.get("metadata", {}).get("namespace"),
            manifest.get("metadata", {}).get("uid"),
            metric_event.sessionCreationTimestamp,
            resource_request_from_manifest(manifest),
            manifest.get("spec", {}).get("jupyterServer", {}).get("image"),
            metric_event.status,
            metric_event.old_status,
            additional_labels_from_manifest(
                metric_event.session,
                additional_label_names,
            ),
        )


class S3RotatingLogHandler(BaseRotatingHandler):
    """Rotating log handler that uploads files to AWS S3 bucket
    when a rotation occurs. After every rotation the copy of the logs is
    not kept locally. The maximum rotation period (in seconds) can be
    specified.
    """

    _datetime_format = "_%Y%m%d_%H%M%S%z"

    def __init__(self, filename, mode, config: S3Config, encoding=None):
        super().__init__(filename, mode, encoding, delay=False)
        self._period_timedelta = timedelta(seconds=config.rotation_period_seconds)
        self._start_timestamp = pytz.UTC.localize(datetime.utcnow())
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
        self._bucket = config.bucket
        self._s3_path_prefix = config.path_prefix
        self.rotator = self._rotator
        self.namer = self._namer
        # NOTE: doRollover persists the data in S3, call at exit
        atexit.register(self.doRollover)

    def _rotator(self, source: str, dest: str):
        os.rename(source, dest)
        self._upload(dest, remove_after_upload=True)

    def _upload(self, fname: str, remove_after_upload: bool = False):
        file_stats = os.stat(fname)
        resp = None
        if file_stats.st_size > 0:
            resp = self._client.upload_file(
                fname, self._bucket, self._s3_path_prefix + "/" + Path(fname).name
            )
        if remove_after_upload:
            os.remove(fname)
        return resp

    def _namer(self, default_name: str) -> str:
        path = Path(default_name)
        new_file = path.parent / (
            path.stem + self._start_timestamp.strftime(self._datetime_format) + path.suffix
        )
        return os.fspath(new_file)

    def doRollover(self):
        if self.stream:
            self.stream.close()
            self.stream = None
        # NOTE: self.rotation_filename calls self.namer
        dfn = self.rotation_filename(self.baseFilename)
        if os.path.exists(dfn):
            os.remove(dfn)
        # NOTE: self.rotate calls self.rotator
        self.rotate(self.baseFilename, dfn)
        self._start_timestamp = pytz.UTC.localize(datetime.utcnow())
        self.stream = self._open()

    def shouldRollover(self, _: str) -> bool:
        now = pytz.UTC.localize(datetime.utcnow())
        if now - self._start_timestamp > self._period_timedelta:
            return True
        return False


class S3Formatter(Formatter):
    """Logging formatter that has ISO8601 timestamps and produces valid json logs."""

    def __init__(self, validate: bool = True) -> None:
        datefmt = "%Y-%m-%dT%H:%M:%S%z"
        style = "%"
        fmt = '{"time":"%(asctime)s", "message":%(message)s}'
        super().__init__(fmt, datefmt, style, validate)

    def formatTime(self, record: LogRecord, datefmt: Optional[str] = None) -> str:
        output = super().formatTime(record, datefmt)
        m = re.match(r"^[\+\-]{1}[0-9]{4}$", output[-5:])
        if m:
            return output[:-2] + ":" + output[-2:]
        # NOTE if the datetime is not timezone aware the match will fail and there
        # is no need to cleanup the time offset to match ISO8601
        return output


class S3MetricHandler(MetricEventHandler):
    """A simple metric handler that persists the metrics
    that are published by Amalthea to a S3 bucket.
    """

    def __init__(
        self,
        logger: Logger,
        config: AuditlogConfig,
    ):
        self.logger = logger
        self._config = config

    def publish(self, metric_event: MetricEvent):
        session_metric_data = SesionMetricData.from_metric_event(
            metric_event,
            additional_label_names=self._config.extra_labels,
        )
        self.logger.info(session_metric_data)
