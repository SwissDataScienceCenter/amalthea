#!/usr/bin/env python

import argparse

from chart_rbac import create_k8s_resources, configure_local_shell


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="""Create the k8s resources liek RBAC roles,
        role binddings, etc that amalthea will
        have when deployed through the helm chart."""
    )
    parser.add_argument(
        "-n",
        "--namespace",
        default="default",
        type=str,
        help="""The namepspace in which we are going to listen for resources.
        Should match the corresponding flag used with `kopf run -n ...` """,
    )
    args = parser.parse_args()
    create_k8s_resources(args.namespace, [args.namespace])
    configure_local_shell(args.namespace)
