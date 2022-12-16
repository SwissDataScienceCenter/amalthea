"""Classes for working with pod and container states.

Usually deals with data like the following:
    containerID: docker://XXXX
    image: some/image:1.2.3
    imageID: docker-pullable://renku/renku-gateway@sha256:XXXX
    lastState:
      terminated:
        containerID: docker://XXXX
        exitCode: 0
        finishedAt: "2022-09-07T11:12:19Z"
        reason: Completed
        startedAt: "2022-09-07T11:11:10Z"
    name: gateway
    ready: true
    restartCount: 2
    started: true
    state:
      running:
        startedAt: "2022-09-07T11:12:19Z"
"""

import logging
from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from itertools import chain
from time import sleep
from typing import Any, ClassVar, Dict, List, Optional, Union

import requests

from controller import config
from controller.server_status_enum import ServerStatusEnum


class K8sPodPhaseEnum(Enum):
    """K8s pod phases based on
    https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-phase"""

    pending: str = "Pending"
    running: str = "Running"
    succeeded: str = "Succeeded"
    failed: str = "Failed"
    unknown: str = "Unknown"


class ContainerTypeEnum(Enum):
    init: str = "initContainers"
    regular: str = "containers"


class K8sContainerStateNamesEnum(Enum):
    """K8s container states based on
    https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#container-states"""

    waiting: str = "waiting"
    running: str = "running"
    terminated: str = "terminated"


@dataclass
class K8sContainerStateWaiting:
    state_name: ClassVar[K8sContainerStateNamesEnum] = K8sContainerStateNamesEnum.waiting
    message: Optional[str] = None
    reason: Optional[str] = None


@dataclass
class K8sContainerStateRunning:
    state_name: ClassVar[K8sContainerStateNamesEnum] = K8sContainerStateNamesEnum.running
    started_at: Optional[str] = None


@dataclass
class K8sContainerStateTerminated:
    state_name: ClassVar[K8sContainerStateNamesEnum] = K8sContainerStateNamesEnum.terminated
    exit_code: Optional[str] = None
    message: Optional[str] = None
    reason: Optional[str] = None


def container_state_factory(
    status: Dict[str, Any], last_state: bool = False
) -> Union[K8sContainerStateRunning, K8sContainerStateWaiting, K8sContainerStateTerminated]:
    states_dict = status.get("lastState" if last_state else "state", {})
    states = list(states_dict.keys())
    if len(states) == 0:
        return K8sContainerStateWaiting()
    if len(states) > 1:
        raise ValueError(f"There can only be one state, found {len(states)}, {states}")
    state = K8sContainerStateNamesEnum(states[0])
    if state == K8sContainerStateNamesEnum.terminated:
        return K8sContainerStateTerminated(
            states_dict[state.value].get("exitCode"),
            states_dict[state.value].get("message"),
            states_dict[state.value].get("reason"),
        )
    if state == K8sContainerStateNamesEnum.running:
        return K8sContainerStateRunning(states_dict[state.value].get("startedAt"))
    return K8sContainerStateWaiting(
        states_dict[state.value].get("message"),
        states_dict[state.value].get("reason"),
    )


@dataclass
class ContainerStatus:
    container_name: str
    ready: bool
    state: Union[K8sContainerStateRunning, K8sContainerStateWaiting, K8sContainerStateTerminated]
    restarts: int
    container_type: ContainerTypeEnum
    last_state: Optional[
        Union[
            K8sContainerStateRunning,
            K8sContainerStateWaiting,
            K8sContainerStateTerminated,
        ]
    ] = None
    restart_limit: int = config.JUPYTER_SERVER_CONTAINER_RESTART_LIMIT

    @classmethod
    def from_k8s_container_status(cls, status: Dict[str, Any], **kwargs):
        return cls(
            container_name=status["name"],
            ready=status.get("ready", False),
            state=container_state_factory(status, last_state=False),
            restarts=int(status.get("restartCount", "0")),
            last_state=container_state_factory(status, last_state=True),
            **kwargs,
        )

    @property
    def failed(self) -> bool:
        if self.completed_successfully:
            return False
        return self.restarts > self.restart_limit

    @property
    def running(self) -> bool:
        return isinstance(self.state, K8sContainerStateRunning)

    @property
    def running_ready(self) -> bool:
        return self.running and self.ready

    @property
    def completed_successfully(self) -> bool:
        return (
            isinstance(self.state, K8sContainerStateTerminated)
            and self.state.exit_code == 0
            and self.ready
        )


class PodConditionsEnum(Enum):
    """
    Pod contidions based on:
    https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-conditions
    """

    initialized: str = "Initialized"  # All init containers done
    scheduled: str = "PodScheduled"  # Pod was scheduled to a node
    has_network: str = "PodHasNetwork"  # Pod networking successfully configured
    containers_ready: str = "ContainersReady"  # All containers in the pod are ready
    ready: str = "Ready"  # Pod is able to serve requests and should reachable by services


@dataclass
class PodCondition:
    type: PodConditionsEnum
    last_transition_time: datetime
    reason: Optional[str] = None
    message: Optional[str] = None
    status: str = "Unknown"

    @classmethod
    def from_dict(cls, condition_dict: Dict[str, str]):
        return cls(
            type=PodConditionsEnum(condition_dict["type"]),
            last_transition_time=datetime.fromisoformat(
                condition_dict["lastTransitionTime"].rstrip("Z")
            ),
            reason=condition_dict.get("reason"),
            message=condition_dict.get("message"),
            status=condition_dict.get("status", "Unknown"),
        )


@dataclass
class ServerStatus:
    pod_phase: K8sPodPhaseEnum
    init_statuses: List[ContainerStatus] = field(default_factory=lambda: [])
    statuses: List[ContainerStatus] = field(default_factory=lambda: [])
    pod_conditions: List[PodCondition] = field(default_factory=lambda: [])
    deletion_timestamp: Optional[datetime] = None
    server_url: Optional[str] = None

    @classmethod
    def from_server_spec(
        cls,
        server: Dict[str, Any],
        init_container_restart_limit: int,
        container_restart_limit: int,
    ):
        init_container_statuses = []
        container_statuses = []
        main_pod_status = server.get("status", {}).get("mainPod", {}).get("status", {})
        for i in main_pod_status.get("initContainerStatuses", []):
            status = ContainerStatus.from_k8s_container_status(
                i,
                restart_limit=init_container_restart_limit,
                container_type=ContainerTypeEnum.init,
            )
            init_container_statuses.append(status)
        for i in main_pod_status.get("containerStatuses", []):
            status = ContainerStatus.from_k8s_container_status(
                i,
                restart_limit=container_restart_limit,
                container_type=ContainerTypeEnum.regular,
            )
            container_statuses.append(status)
        deletion_timestamp = server.get("metadata", {}).get("deletionTimestamp")
        if deletion_timestamp:
            deletion_timestamp = datetime.fromisoformat(deletion_timestamp.rstrip("Z"))
        pod_conditions = main_pod_status.get("conditions", [])
        pod_conditions = [PodCondition.from_dict(condition) for condition in pod_conditions]
        pod_conditions = sorted(
            pod_conditions,
            key=lambda x: x.last_transition_time,
            reverse=True,
        )
        return cls(
            init_statuses=init_container_statuses,
            statuses=container_statuses,
            pod_phase=K8sPodPhaseEnum(main_pod_status.get("phase", "Pending")),
            pod_conditions=pod_conditions,
            deletion_timestamp=deletion_timestamp,
            server_url=server.get("status", {}).get("create_fn", {}).get("fullServerURL"),
        )

    def get_container_summary(self) -> Dict[str, Dict[str, str]]:
        def _get_summary(statuses) -> Dict[str, str]:
            output = {}
            for status in statuses:
                if status.completed_successfully or status.running_ready:
                    output[status.container_name] = "ready"
                elif status.failed:
                    output[status.container_name] = "failed"
                elif status.running:
                    output[status.container_name] = "executing"
                else:
                    output[status.container_name] = "waiting"
            return output

        return {
            "init": _get_summary(self.init_statuses),
            "regular": _get_summary(self.statuses),
        }

    @property
    def is_unschedulable(self) -> bool:
        """Determines is a server pod is unschedulable."""
        return (
            self.pod_phase == K8sPodPhaseEnum.pending
            and len(self.pod_conditions) >= 1
            and self.pod_conditions[0].reason == "Unschedulable"
            # NOTE: every pod is initially unschedulable until a PV is provisioned
            # therefore to avoid "flashing" this state when a sessions starts this case is ignored
            and isinstance(self.pod_conditions[0].message, str)
            and "persistentvolumeclaim" not in self.pod_conditions[0].message.lower()
        )

    def server_url_is_eventually_responsive(self, timeout_seconds: int = 5) -> bool:
        start = datetime.now()
        while True:
            try:
                res = requests.get(self.server_url, timeout=1)
            except (requests.exceptions.RequestException, TimeoutError) as err:
                logging.warning(
                    f"Could not check session full URL {self.server_url} because error: {type(err)}"
                )
            else:
                if res.status_code >= 200 and res.status_code < 400:
                    return True
            if (datetime.now() - start).total_seconds() > timeout_seconds:
                return False
            else:
                sleep(1)
                continue

    @property
    def overall_status(self) -> ServerStatusEnum:
        """Get the status of the jupyterserver."""
        if self.deletion_timestamp:
            return ServerStatusEnum.Stopping
        if self.is_unschedulable:
            return ServerStatusEnum.Failed
        num_failed_statuses = 0
        num_ok_statuses = 0
        for status in chain(self.init_statuses, self.statuses):
            if status.failed:
                num_failed_statuses += 1
            elif status.running_ready or status.completed_successfully:
                num_ok_statuses += 1
        if (
            self.pod_phase == K8sPodPhaseEnum.running
            and num_ok_statuses == len(self.init_statuses) + len(self.statuses)
            and all([condition.status == "True" for condition in self.pod_conditions])
            and self.server_url_is_eventually_responsive()
        ):
            return ServerStatusEnum.Running
        if self.pod_phase == K8sPodPhaseEnum.failed or num_failed_statuses > 0:
            return ServerStatusEnum.Failed
        return ServerStatusEnum.Starting
