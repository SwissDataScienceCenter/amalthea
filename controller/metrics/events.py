from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import datetime
from dateutil import parser
from typing import Any, Dict, Optional

from controller.server_status_enum import ServerStatusEnum


@dataclass
class MetricEvent:
    event_timestamp: datetime
    session: Dict[str, Any]
    old_status: Optional[ServerStatusEnum] = None
    status: Optional[ServerStatusEnum] = None

    def __post_init__(self):
        if self.status and type(self.status) is str:
            self.status = ServerStatusEnum(self.status)
        if self.old_status and type(self.old_status) is str:
            self.old_status = ServerStatusEnum(self.old_status)
        if self.session.get("metadata", {}).get("creationTimestamp"):
            if not self.session.get("metadata"):
                self.session["metadata"] = {}
            self.session["metadata"]["creationTimestamp"] = parser.isoparse(
                self.session.get("metadata", {}).get("creationTimestamp")
            )


class MetricEventHandler(ABC):
    @abstractmethod
    def publish(self, metric_event: MetricEvent):
        pass
