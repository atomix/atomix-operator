build:
	operator-sdk build atomix/atomix-operator:latest
push:
	docker push atomix/atomix-operator:latest
develop:
	export OPERATOR_NAME=atomix-operator
	operator-sdk up local --namespace=default
deploy:
	kubectl create -f deploy/role.yaml
	kubectl create -f deploy/service_account.yaml
	kubectl create -f deploy/role_binding.yaml
	kubectl create -f deploy/operator.yaml
.PHONY: all build push