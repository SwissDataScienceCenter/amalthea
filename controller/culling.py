from datetime import datetime
from json.decoder import JSONDecodeError
from kubernetes.client.rest import ApiException
from kubernetes import config as k8s_config, dynamic
from kubernetes.client import api_client
import logging
import requests
from requests.exceptions import RequestException


def get_cpu_usage(pod, namespace):
    """
    Check the total cpu usage of a pod across all its containers. If the API request to
    get the cpu usage fails (for any reason) report the utilization as zero. This is because
    the culling should not be prevented if the metrics server is not present or cannot be
    found at the expected url.
    """
    total_usage_millicores = 0
    if pod is None:
        return total_usage_millicores
    client = dynamic.DynamicClient(
        api_client.ApiClient(configuration=k8s_config.load_incluster_config())
    )
    try:
        res = client.request(
            "GET", f"/apis/metrics.k8s.io/v1beta1/namespaces/{namespace}/pods/{pod}"
        )
    except ApiException as err:
        logging.warning(
            f"Could not get CPU usage for culling idle sessions for pod {pod}, because: {err}"
        )
        return total_usage_millicores

    try:
        for container in res.containers:
            if container["usage"]["cpu"].endswith("n"):
                total_usage_millicores += int(container["usage"]["cpu"][:-1]) / 1e6
    except (KeyError, ValueError, AttributeError) as err:
        logging.warning(
            f"Could not parse CPU usage for culling idle sessions for pod {pod}, because: {err}"
        )

    return total_usage_millicores


def get_js_server_status(js_body):
    """
    Get the status for the jupyter server from the /api/status endpoint
    by using the body of the jupyter server resource.
    """
    try:
        server_url = js_body["status"]["create_fn"]["fullServerURL"]
        token = js_body["spec"]["auth"].get("token")
    except KeyError:
        return None
    if token is None:
        payload = {}
    else:
        payload = {"token": token}
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
            res["last_activity"].replace("Z", "+00:00")
        )
    if type(res) is dict and "started" in res.keys():
        res["started"] = datetime.fromisoformat(res["started"].replace("Z", "+00:00"))
    return res
