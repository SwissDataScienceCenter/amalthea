from datetime import datetime
from json.decoder import JSONDecodeError
import logging
import pytz
import requests
from requests.exceptions import RequestException

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


def get_js_server_status(js_body):
    """
    Get the status for the jupyter server from the /api/status endpoint
    by using the body of the jupyter server resource.
    """
    try:
        server_url = js_body["status"]["create_fn"]["fullServerURL"]
    except KeyError:
        return None
    payload = (
        {}
        if js_body["spec"]["auth"].get("token") is None
        or js_body["spec"]["auth"].get("token") == ""
        else {"token": js_body["spec"]["auth"].get("token")}
    )
    try:
        res = requests.get(f"{server_url.rstrip('/')}/api/status", params=payload)
    except RequestException as err:
        logging.warning(
            f"Could not get js server status for {server_url}, because: {err}"
        )
        return None

    if res.status_code != 200:
        logging.warning(
            f"Could not get js server status for {server_url}, "
            f"response status code is {res.status_code}"
        )
        return None

    try:
        res = res.json()
    except JSONDecodeError as err:
        logging.warning(
            f"Could not parse js server status for {server_url}, because: {err}"
        )
        return None

    if type(res) is dict and "last_activity" in res.keys():
        res["last_activity"] = datetime.fromisoformat(
            res["last_activity"][:-1] + "+00:00"
            if res["last_activity"].endswith("Z")
            else res["last_activity"]
        ).astimezone(pytz.utc)  # ensure timestamp is UTC
    if type(res) is dict and "started" in res.keys():
        res["started"] = datetime.fromisoformat(
            res["started"][:-1] + "+00:00"
            if res["started"].endswith("Z")
            else res["started"]
        ).astimezone(pytz.utc)  # ensure timestamp is UTC
    return res
