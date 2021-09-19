#  (2021-09-20)


### Features

* publish plain manifests, auto-generate docs from CRD ([af5201b](https://github.com/SwissDataScienceCenter/amalthea/commit/af5201bc33c38ac5ca35cbc456453f274e8e44e6))



# [0.1.0-rc11](https://github.com/SwissDataScienceCenter/amalthea/compare/0.1.0-rc10...0.1.0-rc11) (2021-09-15)


### Bug Fixes

* disable k8s exec probes for jupyter server ([#72](https://github.com/SwissDataScienceCenter/amalthea/issues/72)) ([b44f351](https://github.com/SwissDataScienceCenter/amalthea/commit/b44f351206c24b7e41f81b0c66b7ee37e3f79fbf))



# [0.1.0-rc10](https://github.com/SwissDataScienceCenter/amalthea/compare/0.1.0-rc9...0.1.0-rc10) (2021-09-13)


### Bug Fixes

* fix broken jupyter container probes ([5441c16](https://github.com/SwissDataScienceCenter/amalthea/commit/5441c161b901a8b35d7755155c3d9e48ec4ab0e4))


### Features

* idle sessions culling ([#68](https://github.com/SwissDataScienceCenter/amalthea/issues/68)) ([246d14f](https://github.com/SwissDataScienceCenter/amalthea/commit/246d14f2651ad0c6698c0f9545385d296486aa2c))



# [0.1.0-rc9](https://github.com/SwissDataScienceCenter/amalthea/compare/0.1.0-rc6...0.1.0-rc9) (2021-09-07)


### Bug Fixes

* authorization/Dockerfile to reduce vulnerabilities ([cca3f52](https://github.com/SwissDataScienceCenter/amalthea/commit/cca3f52dee5b0a45dc86d93ccf26aab5e2ae6e1e))
* authorization/Dockerfile to reduce vulnerabilities ([0985c12](https://github.com/SwissDataScienceCenter/amalthea/commit/0985c121192f869668d52c52deda1af9919a24e1))
* cookie-cleaner/Dockerfile to reduce vulnerabilities ([1ee3525](https://github.com/SwissDataScienceCenter/amalthea/commit/1ee35256022b40c99235db77f1cd344ee362f24f))
* cookie-cleaner/Dockerfile to reduce vulnerabilities ([bdaa6b2](https://github.com/SwissDataScienceCenter/amalthea/commit/bdaa6b27f026cb66857091edec77711e176fcd15))
* don't overwrite the default location for jupyter config ([29ccd46](https://github.com/SwissDataScienceCenter/amalthea/commit/29ccd46f56420d9b3fb0b8529869bf19f9f1f1c2))
* fix acceptance tests, use jupyter base-notebook for testing ([b0d5067](https://github.com/SwissDataScienceCenter/amalthea/commit/b0d5067795e02d698c3600e094c7ac5c402fb855))
* pass host header to jupyter server, make server tolerate it ([ee6c31a](https://github.com/SwissDataScienceCenter/amalthea/commit/ee6c31a3654c36403d6332cdc16145dcf3226256))
* remove service links from user pod ([2f9f6de](https://github.com/SwissDataScienceCenter/amalthea/commit/2f9f6dec5124b9835853439268ab36187b62b7a5))


### Features

* add probes to the jupyter server container ([2339443](https://github.com/SwissDataScienceCenter/amalthea/commit/2339443fe76d7ff66e44a4322a9a2aa918e543f2))



# [0.1.0-rc6](https://github.com/SwissDataScienceCenter/amalthea/compare/0.1.0-rc3...0.1.0-rc6) (2021-08-11)


### Bug Fixes

* adapt chart values schema ([503b8cf](https://github.com/SwissDataScienceCenter/amalthea/commit/503b8cf93564a6d3b817a36575269f8b6d36f586))
* Kopf error handling during resource creation ([#52](https://github.com/SwissDataScienceCenter/amalthea/issues/52)) ([0b12a43](https://github.com/SwissDataScienceCenter/amalthea/commit/0b12a43bbd451384af1d5cf6e4eece37910ae3c7))
* remove args and command from jupyterserver container ([#48](https://github.com/SwissDataScienceCenter/amalthea/issues/48)) ([3a563e8](https://github.com/SwissDataScienceCenter/amalthea/commit/3a563e826835eb8762eea704bdfd5ac43001681e))
* use networking api version for k8s ingress ([#49](https://github.com/SwissDataScienceCenter/amalthea/issues/49)) ([69397f8](https://github.com/SwissDataScienceCenter/amalthea/commit/69397f8639282b6c83c91feef7db20f45be38b38))
* watch all kinds of resources explicitly ([d52f899](https://github.com/SwissDataScienceCenter/amalthea/commit/d52f899af3fb3233b086322335494847273a4ecb))


### Features

* add acceptance tests ([#44](https://github.com/SwissDataScienceCenter/amalthea/issues/44)) ([0be824b](https://github.com/SwissDataScienceCenter/amalthea/commit/0be824b20ea5f48d84b512cce5b2afdee6504b41))
* use minimal roles for development and testing ([8daa8d5](https://github.com/SwissDataScienceCenter/amalthea/commit/8daa8d5468d74aeb25550a752471e1bb686ff70d))



# [0.1.0-rc3](https://github.com/SwissDataScienceCenter/amalthea/compare/0.1.0-rc1...0.1.0-rc3) (2021-07-08)


### Bug Fixes

* allow global property in values for helm chart ([#42](https://github.com/SwissDataScienceCenter/amalthea/issues/42)) ([278009b](https://github.com/SwissDataScienceCenter/amalthea/commit/278009bc2c015ec589d2640278ef5878eb3b959a))


### Features

* add tests ([#39](https://github.com/SwissDataScienceCenter/amalthea/issues/39)) ([b115f87](https://github.com/SwissDataScienceCenter/amalthea/commit/b115f876ba285d68c4253a2fa0c6ccdc1cf8fa64))
* add validation for values.yaml and make amalthea watch resources in helm deployment namespace by default ([#41](https://github.com/SwissDataScienceCenter/amalthea/issues/41)) ([8fb4a70](https://github.com/SwissDataScienceCenter/amalthea/commit/8fb4a7040d7eb6075f39dd4a6a5b6d3f4e068b61))



# [0.1.0-rc1](https://github.com/SwissDataScienceCenter/amalthea/compare/da735ced323eacb38fd010e4ae0a0479fb2bf310...0.1.0-rc1) (2021-06-25)


### Bug Fixes

* address comments ([c7ded42](https://github.com/SwissDataScienceCenter/amalthea/commit/c7ded42717687446aa6e6380d38af825654cb114))
* avoid failing probe during startup ([6e615c5](https://github.com/SwissDataScienceCenter/amalthea/commit/6e615c506e72af3f4b155b80c1f7b6c8492b9464))
* limit RBAC to the necessary ([cfe58a3](https://github.com/SwissDataScienceCenter/amalthea/commit/cfe58a373fbad0c2bd814927d772e276cce8c3f4))
* replace notebook cookie secret value with file ([#36](https://github.com/SwissDataScienceCenter/amalthea/issues/36)) ([4bab232](https://github.com/SwissDataScienceCenter/amalthea/commit/4bab232f38c931d2d1dab704c1dcae150e33dcf8))
* set ingress tls secret and annotations in crd ([fbc747d](https://github.com/SwissDataScienceCenter/amalthea/commit/fbc747d3ba94f6f74153c0ae2295c80cfd16e7a3))
* typo in api version name for crd ([#38](https://github.com/SwissDataScienceCenter/amalthea/issues/38)) ([0b1d704](https://github.com/SwissDataScienceCenter/amalthea/commit/0b1d70420407f49b49768c777db4f90be02d172e))
* use python secrets for oauth cookie secret ([#37](https://github.com/SwissDataScienceCenter/amalthea/issues/37)) ([dc54ee0](https://github.com/SwissDataScienceCenter/amalthea/commit/dc54ee0d04a3c24feb17b6694f99970de9cc7ad8))
* **operator:** configure client-side watch stream timeout ([03cae0c](https://github.com/SwissDataScienceCenter/amalthea/commit/03cae0c35385721d72ae4846499a21d56fb72a49))


### Features

* **chart:** template CRD name and API group/version ([a5beb39](https://github.com/SwissDataScienceCenter/amalthea/commit/a5beb39b345b3e7c09304f8bb3137e6c5946a943))
* add child stati to custom object ([2e8a8c3](https://github.com/SwissDataScienceCenter/amalthea/commit/2e8a8c37d569ad72a0ca9cbf9f0571a78d14a91c))
* add cookie white- and blacklisting ([c79142a](https://github.com/SwissDataScienceCenter/amalthea/commit/c79142ad77cbdf81d8dd5d4f144ac99c2398dc14))
* add empty dir storage for sessions ([3abcb61](https://github.com/SwissDataScienceCenter/amalthea/commit/3abcb6172ba7d07312d620c1246941747e01f8a3))
* add liveness probe ([1e5b295](https://github.com/SwissDataScienceCenter/amalthea/commit/1e5b295ab628e04d18ff7484a98e8caaba78a7e8))
* add networkpolicies, refactor labling ([0a9b8e2](https://github.com/SwissDataScienceCenter/amalthea/commit/0a9b8e2c4a35aadb5f0f2ee4e3541703bea53d09))
* better handling of secrets ([aff9139](https://github.com/SwissDataScienceCenter/amalthea/commit/aff913904fb12c402515978456ed283487ae7eb5))
* enable json merge patches ([cb589a8](https://github.com/SwissDataScienceCenter/amalthea/commit/cb589a86ac1d26ce5fcf6cf542024adc76a77056))
* enable non-TLS sessions ([0217cb8](https://github.com/SwissDataScienceCenter/amalthea/commit/0217cb8615feefe6eb004b85f6152414dd6e5035))
* initial commit of sanitized poc ([da735ce](https://github.com/SwissDataScienceCenter/amalthea/commit/da735ced323eacb38fd010e4ae0a0479fb2bf310))
* more generic patching and customization ([0210df7](https://github.com/SwissDataScienceCenter/amalthea/commit/0210df7ad408df3ec32295d34b0b1a38adc4e3c2))
* several improvements and refactoring ([8687261](https://github.com/SwissDataScienceCenter/amalthea/commit/8687261039f345129aa62b9e68ae84a4a35e3483))



