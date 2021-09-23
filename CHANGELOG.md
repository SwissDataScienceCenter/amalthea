# [0.1.0](https://github.com/SwissDataScienceCenter/amalthea/compare/da735ced323eacb38fd010e4ae0a0479fb2bf310...0.1.0) (2021-09-15)


### Features

* Use a k8s operator to run Jupyter servers
* Define new k8s resource called JupyterServer that contains information about the image used in the server, routing and authentication
* Ability to use OIDC authentication or define a simple token
* Enable the use of JSON patching to change or add to any aspect of the JupyterServer resources
* Optionally use a specific k8s scheduler with a custom strategy for the JupyterServers
* Use either k8s persistent volumes or `emptyDir` storage for users' data
* Culling of inactive servers with the option to define an idleness threshold on a per server basis
* Use stateful sets instead of plain pods for better resilience and recovery in the event of node failures
* Available as a helm chart or plain manifests
