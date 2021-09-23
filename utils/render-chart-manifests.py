import os
import shutil
import subprocess

import yaml

"""
Simple script which renders the helm chart with the default values
and removes some helm specific annotations. For convenience, we also
generate a kustomization file.
"""

if __name__ == "__main__":

    repo_root = (
        subprocess.check_output("git rev-parse --show-toplevel", shell=True)
        .decode()
        .strip()
    )

    rendered_chart = subprocess.check_output(
        f"helm template amalthea -n default {repo_root}/helm-chart/amalthea",
        shell=True,
    ).decode()

    lines = rendered_chart.splitlines()
    filtered_lines = [line for line in lines if "helm.sh/chart:" not in line]
    filtered_lines = [
        line
        for line in filtered_lines
        if "app.kubernetes.io/managed-by: Helm" not in line
    ]
    all_manifests = "\n".join(filtered_lines)
    manifests = all_manifests.split("---")

    kustomization = {
        "apiVersion": "kustomize.config.k8s.io/v1beta1",
        "kind": "Kustomization",
        "resources": [],
    }

    # empty the manifests directory
    shutil.rmtree(f"{repo_root}/manifests/", ignore_errors=True)
    os.mkdir(f"{repo_root}/manifests/")

    for manifest in manifests:
        manifest = manifest.lstrip("\n")
        if len(manifest) == 0:
            continue
        template_path = manifest.splitlines()[0].split("# Source: amalthea/templates/")[
            1
        ]
        out_filename = f"{repo_root}/manifests/{template_path}"
        try:
            os.makedirs(os.path.dirname(out_filename))
        except FileExistsError:
            pass
        with open(out_filename, "a+") as outfile:
            outfile.write("---\n")
            outfile.write(
                "# This manifest is auto-generated from the helm chart, do not modify! \n"
            )
            outfile.write(manifest)
        kustomization["resources"].append(f"./{template_path}")

    # some resources are inside the same file
    kustomization["resources"] = sorted(list(set(kustomization["resources"])))

    with open(f"{repo_root}/manifests/kustomization.yaml", "w") as kustomization_file:
        yaml.dump(kustomization, kustomization_file)
