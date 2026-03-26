IMAGE = us-west2-docker.pkg.dev/xsolla-school-experiments/ahmed-zidan/checkout-api
TAG   = v1.0.1

.PHONY: build push deploy undeploy

## Build the image for linux/amd64 (GKE compatible)
build:
	docker buildx build --platform linux/amd64 -t $(IMAGE):$(TAG) --load .

## Push the image to GCP Artifact Registry
push:
	docker push $(IMAGE):$(TAG)

## Build and push in one step
release: build push

## Apply all k8s manifests
deploy:
	kubectl apply -f k8s/namespace.yaml
	kubectl apply -f k8s/secret.yaml
	kubectl apply -f k8s/postgres/statefulset.yaml
	kubectl apply -f k8s/postgres/service.yaml
	kubectl apply -f k8s/deployment.yaml
	kubectl apply -f k8s/service.yaml
	kubectl apply -f k8s/ingress.yaml

## Delete all k8s resources
undeploy:
	kubectl delete -f k8s/ -R

## Show status of all resources in the checkout namespace
status:
	kubectl get all -n checkout
