## Note that the changelog has moved exclusively to https://github.com/SwissDataScienceCenter/amalthea/releases

## [0.12.3](https://github.com/SwissDataScienceCenter/amalthea/compare/0.12.2...0.12.3) (2024-08-10)


### Bug Fixes


* metrics and auditlog config ([2cf0758](https://github.com/SwissDataScienceCenter/amalthea/commit/2cf075887522681367e53c479c7384a6c59a0f62))



## [0.12.2](https://github.com/SwissDataScienceCenter/amalthea/compare/0.12.1...0.12.2) (2024-08-07)


### Bug Fixes

* add kopf liveness check ([3c4daa1](https://github.com/SwissDataScienceCenter/amalthea/commit/3c4daa1420f6faab031d87be95198e8647771a0f))



## [0.12.1](https://github.com/SwissDataScienceCenter/amalthea/compare/0.12.0...0.12.1) (2024-08-06)


### Bug Fixes

* properly pass down namespace configuration ([#668](https://github.com/SwissDataScienceCenter/amalthea/issues/668)) ([aee87e0](https://github.com/SwissDataScienceCenter/amalthea/commit/aee87e0b0a8965735fa081c8299b20575731f1d0))



## [0.12.0](https://github.com/SwissDataScienceCenter/amalthea/compare/0.11.0...0.12.0) (2024-08-05)


### Bug Fixes

* Change the default registry to pull kube-scheduler from. ([#557](https://github.com/SwissDataScienceCenter/amalthea/issues/557)) ([e21e99f](https://github.com/SwissDataScienceCenter/amalthea/commit/e21e99f28e65f4cc1c426949d6965817aa1978de))
* parse unknown or new k8s pod conditions ([#553](https://github.com/SwissDataScienceCenter/amalthea/issues/553)) ([45780c6](https://github.com/SwissDataScienceCenter/amalthea/commit/45780c6a6e32391b1bf740f423a748b39d85d970))


### Features

* add csi-rclone as dependency ([#588](https://github.com/SwissDataScienceCenter/amalthea/issues/588)) ([6c08a05](https://github.com/SwissDataScienceCenter/amalthea/commit/6c08a05254da556a518dcd03f536e2251b0656d6))
* **addEnvFrom:** possibility to inject env variables ([#579](https://github.com/SwissDataScienceCenter/amalthea/issues/579)) ([97b2d87](https://github.com/SwissDataScienceCenter/amalthea/commit/97b2d87ea5f2933a80d37a15f26428752f8f59da))
* **networkpolicies:** customize cluster IPs ([#580](https://github.com/SwissDataScienceCenter/amalthea/issues/580)) ([6b374b3](https://github.com/SwissDataScienceCenter/amalthea/commit/6b374b356297181629e70d3b71a7b6ccb5c59272))


### Features for brand new operator and CRDs (AmaltheaSession)

Note this code is not operational and it is not deployed or publised anywhere yet.

* AmaltheaSession CRD spec in golang ([#585](https://github.com/SwissDataScienceCenter/amalthea/issues/585)) ([e8f765d](https://github.com/SwissDataScienceCenter/amalthea/commit/e8f765d44b92eda787837e091e7d17264f5696e3))
* basic status and reconcile implementation ([#605](https://github.com/SwissDataScienceCenter/amalthea/issues/605)) ([dd1f6ab](https://github.com/SwissDataScienceCenter/amalthea/commit/dd1f6ab1386384c93aad66688aac030696a6d53a))
* **crd:** AmaltheaSession definition ([#560](https://github.com/SwissDataScienceCenter/amalthea/issues/560)) ([a89bad7](https://github.com/SwissDataScienceCenter/amalthea/commit/a89bad7aeef8a3205d063dcf5771b85440ac0115))
* expand the CR spec with additional vols ([#618](https://github.com/SwissDataScienceCenter/amalthea/issues/618)) ([430dc53](https://github.com/SwissDataScienceCenter/amalthea/commit/430dc53d06081e157e7d050df20f871511039a52))
* implement core AmaltheaSession crd elements creation ([#596](https://github.com/SwissDataScienceCenter/amalthea/issues/596)) ([d710bcf](https://github.com/SwissDataScienceCenter/amalthea/commit/d710bcf8cea9788137a268ae42e7a816123c963c))
* Scaffold go operator framework ([#582](https://github.com/SwissDataScienceCenter/amalthea/issues/582)) ([a4e299f](https://github.com/SwissDataScienceCenter/amalthea/commit/a4e299fda93055c499e2ade2b4c5529780b0adb6))


## [0.11.0](https://github.com/SwissDataScienceCenter/amalthea/compare/0.10.0...0.11.0) (2024-01-16)

### Bug Fixes

* **app:** failed status should not override hibernated status ([#494](https://github.com/SwissDataScienceCenter/amalthea/issues/494)) ([8c16330](https://github.com/SwissDataScienceCenter/amalthea/commit/8c16330fb7ea6ab940f77a95dc62eee22be034b6))


### Features

* **app:** enable patching of server resources ([#459](https://github.com/SwissDataScienceCenter/amalthea/issues/459)) ([764b199](https://github.com/SwissDataScienceCenter/amalthea/commit/764b19942bd37911edf649ae88c44ada96c54488))


## [0.10.0](https://github.com/SwissDataScienceCenter/amalthea/compare/0.9.1...0.10.0) (2023-10-23)

This release has a breaking change in the values file for the Amalthea Helm chart. This is only relevant
if you are using the Amalthea scheduler. Please refer to the values file of this release for additional 
information. All the values from the old version under the `scheduler` key have been removed and replaced
with new values, so if you wish to retain similar scheduler functionality like before you have to change your
values file when upgrading.

### Bug Fixes

* **chart:** adapt scheduler for new k8s versions ([#419](https://github.com/SwissDataScienceCenter/amalthea/issues/419)) ([029486e](https://github.com/SwissDataScienceCenter/amalthea/commit/029486e071ba200c802f636379a0c7b29c79f67e))

## [0.9.1](https://github.com/SwissDataScienceCenter/amalthea/compare/0.9.0...0.9.1) (2023-09-06)

### Bug Fixes

* clear hibernation annotations when resuming pods to avoid a flicker in UI ([#383](https://github.com/SwissDataScienceCenter/amalthea/issues/383)) ([7819332](https://github.com/SwissDataScienceCenter/amalthea/commit/78193320d8e587d89a613de8bd670e48196c69ee))



## [0.9.0](https://github.com/SwissDataScienceCenter/amalthea/compare/0.8.0...0.9.0) (2023-08-31)

### Features

* add two-step culling for sessions ([#366](https://github.com/SwissDataScienceCenter/amalthea/issues/366)) ([bfd88f3](https://github.com/SwissDataScienceCenter/amalthea/commit/bfd88f3a0c3f65dca929cadaafaf14f570443559))



## [0.8.0](https://github.com/SwissDataScienceCenter/amalthea/compare/0.7.1...0.8.0) (2023-08-03)

### Features

* update CRD to support session hibernation ([#347](https://github.com/SwissDataScienceCenter/amalthea/issues/347)) ([64b65d7](https://github.com/SwissDataScienceCenter/amalthea/commit/64b65d7ed260a7d5103d61a26228c6e912d4dc22))



## [0.7.1](https://github.com/SwissDataScienceCenter/amalthea/compare/0.7.0...0.7.1) (2023-07-18)

### Bug Fixes

* **app:** upgrade OAuth2 proxy to 7.4.0 to prevent unnecessary proxy restarting ([#341](https://github.com/SwissDataScienceCenter/amalthea/issues/341)) ([091909f](https://github.com/SwissDataScienceCenter/amalthea/commit/091909fde471992c007141db842988b1234a7aea))

## [0.7.0](https://github.com/SwissDataScienceCenter/amalthea/compare/0.6.1...0.7.0) (2023-06-27)

### Features

* **app:** add error and update status when resource quota is exceeded ([#336](https://github.com/SwissDataScienceCenter/amalthea/issues/336)) ([cf6e7ca](https://github.com/SwissDataScienceCenter/amalthea/commit/cf6e7ca788359d6544f4b27319a34f4e9e246d39))



## [0.6.1](https://github.com/SwissDataScienceCenter/amalthea/compare/0.6.0...0.6.1) (2023-02-24)


### Bug Fixes

* **app:** consider idle when connections are <= 0 ([#314](https://github.com/SwissDataScienceCenter/amalthea/issues/314)) ([edafb45](https://github.com/SwissDataScienceCenter/amalthea/commit/edafb45f28ab55ec2e03ff8af60c113b1fbdb97a))



## [0.6.0](https://github.com/SwissDataScienceCenter/amalthea/compare/0.5.2...0.6.0) (2022-10-12)


### Bug Fixes

* **app:** do not use previous status for state ([#243](https://github.com/SwissDataScienceCenter/amalthea/issues/243)) ([ec78326](https://github.com/SwissDataScienceCenter/amalthea/commit/ec7832674538a0788f79996ff30faf59179f351b))


### Features

* detailed status messages ([#258](https://github.com/SwissDataScienceCenter/amalthea/issues/258)) ([95371dd](https://github.com/SwissDataScienceCenter/amalthea/commit/95371ddea8e09cdbccfed658a55f85ea73094cad))



## [0.5.2](https://github.com/SwissDataScienceCenter/amalthea/compare/0.5.1...0.5.2) (2022-08-11)


### Bug Fixes

* **metrics:** buckets for prometheus histogram metrics ([#189](https://github.com/SwissDataScienceCenter/amalthea/issues/189)) ([7b34872](https://github.com/SwissDataScienceCenter/amalthea/commit/7b3487207d902919e2934fda7654850d20cdd903))
* **metrics:** do not publish same metric more than once ([#190](https://github.com/SwissDataScienceCenter/amalthea/issues/190)) ([148d214](https://github.com/SwissDataScienceCenter/amalthea/commit/148d214a01f8ab97008b18c1b1089c481337d047))



## [0.5.1](https://github.com/SwissDataScienceCenter/amalthea/compare/0.5.0...0.5.1) (2022-07-22)


### Bug Fixes

* **app:** prevent flashing error message on startup ([#182](https://github.com/SwissDataScienceCenter/amalthea/issues/182)) ([ce7e809](https://github.com/SwissDataScienceCenter/amalthea/commit/ce7e80935d94fb4a03a1430bb56019e8082a109c))
* **general**: upgrade base image in Dockerfile to reduce vulnerabilities ([#183](https://github.com/SwissDataScienceCenter/amalthea/issues/183)) ([c975ef0](https://github.com/SwissDataScienceCenter/amalthea/commit/c975ef0e53e9baa2c938fd9c3510b014ddc7b917))



## [0.5.0](https://github.com/SwissDataScienceCenter/amalthea/compare/0.4.0...0.5.0) (2022-07-04)


### Features

* **app:** long term metric storage ([#173](https://github.com/SwissDataScienceCenter/amalthea/issues/173)) ([6335ec5](https://github.com/SwissDataScienceCenter/amalthea/commit/6335ec5b3657e5d8dba1adb239b9f9e87dbf428b))


## [0.4.0](https://github.com/SwissDataScienceCenter/amalthea/compare/0.3.0...0.4.0) (2022-06-13)

### Bug Fixes

* **app:** avoid temporary fail state when starting ([#168](https://github.com/SwissDataScienceCenter/amalthea/issues/168)) ([46e0d8d](https://github.com/SwissDataScienceCenter/amalthea/commit/46e0d8d9486c78b6114dd2ab74cadd7da0cb92eb))


### Features

* **app:** add and modify prometheus metrics ([#164](https://github.com/SwissDataScienceCenter/amalthea/issues/164)) ([1f34d84](https://github.com/SwissDataScienceCenter/amalthea/commit/1f34d84ab8fcaff7654f149d8e67d4166bf01771))
* **app:** cull sessions pending too long ([#158](https://github.com/SwissDataScienceCenter/amalthea/issues/158)) ([8fc359e](https://github.com/SwissDataScienceCenter/amalthea/commit/8fc359ea83e9c643b1b7c34b2573c5e829baa35f))


## [0.3.0](https://github.com/SwissDataScienceCenter/amalthea/compare/0.2.3...0.3.0) (2022-05-16)

### Bug Fixes

* **chart:** allow number or string for disk size in CRD ([#146](https://github.com/SwissDataScienceCenter/amalthea/issues/146)) ([8351f29](https://github.com/SwissDataScienceCenter/amalthea/commit/8351f29163dacec2af729f69f832dc8e40357773))
* **app:** use group in dynamic k8s client ([#151](https://github.com/SwissDataScienceCenter/amalthea/issues/151)) ([31b5de1](https://github.com/SwissDataScienceCenter/amalthea/commit/31b5de11ffc4f889ee7bbdcc5c4cf31df10addd0))
* **test:** cleanup of k8s resources in fixtures ([#144](https://github.com/SwissDataScienceCenter/amalthea/issues/144)) ([d632170](https://github.com/SwissDataScienceCenter/amalthea/commit/d6321700bfc78a4080064a8697ce6f7eb8b8e773))

### Features

* **app:** expose metrics to prometheus  ([#145](https://github.com/SwissDataScienceCenter/amalthea/issues/145)) ([a109b77](https://github.com/SwissDataScienceCenter/amalthea/commit/a109b77741eaac9aa9c6b19a8d553e205ae57e38))


## [0.2.3](https://github.com/SwissDataScienceCenter/amalthea/compare/0.2.2...0.2.3) (2022-01-04)



### Bug Fixes

* **app:** Optional user-scheduler ([43ad69c](https://github.com/SwissDataScienceCenter/amalthea/commit/43ad69ca639acb90470abafce46005f8ee20fc3c))



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
