from queue import Queue
from typing import List
import threading

from controller.metrics.events import MetricEventHandler, MetricEvent


class MetricsQueue:
    def __init__(self, metric_handlers=List[MetricEventHandler]):
        self.q = Queue()
        self.metric_handlers = metric_handlers
        self.thread = None

    def _queue_worker(self):
        while True:
            metric_event = self.q.get()
            for handler in self.metric_handlers:
                handler.publish(metric_event)

    def start_workers(self):
        if len(self.metric_handlers) == 0:
            return
        self.thread = threading.Thread(target=self._queue_worker, daemon=True)
        self.thread.start()

    def add_to_queue(self, metric_event: MetricEvent):
        if len(self.metric_handlers) == 0:
            return
        self.q.put(metric_event)
