#  (2021-11-30)



## [0.2.2](https://github.com/SwissDataScienceCenter/amalthea/compare/0.2.1...0.2.2) (2021-11-30)



### Bug Fixes

* **app:** probes and culling ([#127](https://github.com/SwissDataScienceCenter/amalthea/issues/127)) ([3c02584](https://github.com/SwissDataScienceCenter/amalthea/commit/3c02584eb3913f6329a4f736a41070005f9d3ad9))



## [0.2.1](https://github.com/SwissDataScienceCenter/amalthea/compare/0.2.0...0.2.1) (2021-11-16)



### Bug Fixes

* **app:** failing probes for anonymous sessions ([#122](https://github.com/SwissDataScienceCenter/amalthea/issues/122)) ([df96164](https://github.com/SwissDataScienceCenter/amalthea/commit/df96164bd44dd68d7fbf904db9bcdac72aab7cae))



## [0.2.0](https://github.com/SwissDataScienceCenter/amalthea/compare/0.1.3...0.2.0) (2021-11-12)



### Bug Fixes

* **app:** failing probes for rstudio ([#117](https://github.com/SwissDataScienceCenter/amalthea/issues/117)) ([4fc45f6](https://github.com/SwissDataScienceCenter/amalthea/commit/4fc45f6855e485174b01d68efc0f07f6ebcd88b3))
* **chart:** allow egress from sessions to any port out of cluster ([#119](https://github.com/SwissDataScienceCenter/amalthea/issues/119)) ([49c7a62](https://github.com/SwissDataScienceCenter/amalthea/commit/49c7a6219dc7de3b511d526b0d0ab7d8d196bc2a))


### Features

* **app:** add resource usage to JupyterServer resources ([#104](https://github.com/SwissDataScienceCenter/amalthea/issues/104)) ([e4fc65e](https://github.com/SwissDataScienceCenter/amalthea/commit/e4fc65ea7a9c2816bec6c6c4224316b0d6de052b))



## [0.1.3](https://github.com/SwissDataScienceCenter/amalthea/compare/0.1.2...0.1.3) (2021-11-08)


### Bug Fixes

* **app:** faililng k8s probes for rstudio ([#112](https://github.com/SwissDataScienceCenter/amalthea/issues/112)) ([7aa13a5](https://github.com/SwissDataScienceCenter/amalthea/commit/7aa13a517721473d6a85c30c744c60fa3dc74b75))



## [0.1.2](https://github.com/SwissDataScienceCenter/amalthea/compare/0.1.1...0.1.2) (2021-10-22)


### Features

* **app:** option to not limit size of user session emptyDir ([#110](https://github.com/SwissDataScienceCenter/amalthea/issues/110)) ([47a9631](https://github.com/SwissDataScienceCenter/amalthea/commit/47a96312e2e86b8e44e6f6e77964c19f82e956b9))



## [0.1.1](https://github.com/SwissDataScienceCenter/amalthea/compare/0.1.0...0.1.1) (2021-10-14)


### Bug Fixes

* react to child resource events without an event type ([6fb4065](https://github.com/SwissDataScienceCenter/amalthea/commit/6fb4065f4a693aa9cceac86425228335b2cfd2f8))


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
