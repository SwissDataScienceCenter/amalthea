from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import datetime
from typing import Any, Dict, Optional

from controller.server_status_enum import ServerStatusEnum


@dataclass
class MetricEvent:
    event_timestamp: datetime
    session: Dict[str, Any]
    old_status: Optional[ServerStatusEnum] = None


class MetricEventHandler(ABC):
    @abstractmethod
    def publish(self, metric_event: MetricEvent):
        pass
