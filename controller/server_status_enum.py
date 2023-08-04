from enum import Enum

from typing import Optional


class ServerStatusEnum(Enum):
    """Simple Enum for server status."""

    Running = "running"
    Starting = "starting"
    Stopping = "stopping"
    Failed = "failed"
    Hibernated = "hibernated"

    @classmethod
    def list(cls):
        return list(map(lambda c: c.value, cls))

    @classmethod
    def from_string(cls, status: Optional[str]) -> Optional["ServerStatusEnum"]:
        return None if status is None else cls(status)
