from collections import namedtuple
import requests
from urllib.parse import urlunparse, urlparse
import logging

from controller.utils import get_pod_metrics, parse_pod_metrics


def get_cpu_usage_for_culling(pod, namespace):
    """
    Check the total cpu usage of a pod across all its containers. If the API request to
    get the cpu usage fails (for any reason) report the utilization as being 0.
    This is because the culling should be done even if the metrics server is not present
    or cannot be found at the expected url.
    """
    total_default_usage_millicores = 0
    total_usage_millicores = 0
    found_metrics = False
    if pod is None:
        return total_default_usage_millicores
    metrics = get_pod_metrics(pod, namespace)
    if metrics is None:
        return total_default_usage_millicores
    parsed_metrics = parse_pod_metrics(metrics)

    for container in parsed_metrics:
        if "cpu_millicores" in container.keys():
            found_metrics = True
            total_usage_millicores += container["cpu_millicores"]

    if found_metrics:
        return total_usage_millicores
    else:
        return total_default_usage_millicores


def is_idle_probe_idle(spec) -> bool:
    """
    Check the idle probe (if defined) and determine whether the session is
    idle (True) or active (False).
    """
    # NOTE: By definition a value of 0 for the threshold means that the
    # sesion will never be culled due to idleness. If this is the case return False
    # to indicate the server is "active".
    if spec["culling"]["idleSecondsThreshold"] == 0:
        return False
    host = spec["culling"]["idleProbe"]["httpGet"].get("host")
    if host is None:
        # INFO: host is not defined in spec, default to main pod IP
        main_pod_ip = spec.get("status", {}).get("mainPod", {}).get("status", {}).get("podIP")
        if main_pod_ip is None:
            # INFO: main pod IP not present in status, assume session is starting up
            # and therefore not idle
            return False
        host = main_pod_ip
    headers = {
        i["name"]: i["value"] for i in spec["culling"]["idleProbe"]["httpGet"]["httpHeaders"]
    }
    UrlParseResult = namedtuple(
        "ParseResult",
        ["scheme", "netloc", "path", "params", "query", "fragment"],
    )
    if spec["culling"]["idleProbe"]["httpGet"].get("port", False):
        netloc = host + ":" + spec["culling"]["idleProbe"]["httpGet"]["port"]
    parsed_path = urlparse(spec["culling"]["idleProbe"]["httpGet"]["path"])
    url = urlunparse(UrlParseResult(
        spec["culling"]["idleProbe"]["httpGet"]["scheme"],
        netloc,
        parsed_path.path,
        parsed_path.params,
        parsed_path.query,
        parsed_path.fragment,
    ))
    res = requests.get(
        url=url,
        headers=headers,
        allow_redirects=False,
    )
    logging.info(f"Sending GET request to url: {url} with headers {headers}")
    logging.info(f"Response from idleProbe GET request is {res.status_code}")
    # INFO: Logic is similar to livenessProbe in k8s. For a livenessProbe a value in the range
    # >=200 and <400 indicates that the session is alive. For an idleProbe a value in this
    # range indicates that the session is idle.
    return res.status_code >= 200 and res.status_code < 400
