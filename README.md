# Basic (Partial) Implementation of Amalthea in Go

Note: This is not production ready or intended to replace the current implementation. It is just a side-project for learning Go.

Hopefully we never get to this point - but if we ever need to re-implement Amalthea in Go this should be a good starting point.

## How to run:
Get a k8s context - kind is easiest
1. `go mod tidy`
2. `make install run`
3. `kubectl apply -f simple-example.yaml`

## What is missing:
- culling
- logic about properly handling secrets for the Jupyter server - right now all secrets are just randomly generated regardless of what you specify
- code organization / structure is probably pretty bad / ugly
- status updates
- proper owner references for child resources - right now all are hardcoded in the templates which means patching in brand new resources will not mark them as children of the server
- helm chart
- tests
- CI pipeline
- CRD docs
- because of Go being strict about types and me not being very knowledgeable the patches work but they have to be defined as strings in the manifest - there is probably a way to handle this better however - nevertheless the patches work as expected
