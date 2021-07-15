#!/usr/bin/env python

import argparse

from chart_rbac import configure_local_dev


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="""Configure your kubectl context to use a
        service account with the RBAC roles that amalthea will
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
    configure_local_dev(args.namespace, [args.namespace])
