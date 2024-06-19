.PHONY: crd crd_check style_checks run tests kind_cluster

crd:
	poetry run python -m controller.crds template

crd_check:
	poetry run python -m controller.crds check

style_checks:
	poetry run flake8 ./
	poetry run black --check ./

run:
	poetry run python -m controller.main

tests:
	poetry run pytest

kind_cluster:
	kind delete cluster
	kind create cluster --config kind_config.yaml
	kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
	echo "Waiting for ingress controller to initialize"
	sleep 15
	kubectl wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=90s

