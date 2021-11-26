ifndef DOCKER_IMAGE_REPO
  DOCKER_IMAGE_REPO=gopaddle/configurator
endif

ifndef DOCKER_IMAGE_TAG
  DOCKER_IMAGE_TAG=new
endif

.PHONY: helm

clean: clean-configurator
build: build-configurator
push: push-image
deploy: deploy-configurator deploy-crds
remove: remove-configurator cleanup
cleanup: cleanup-crds

clean-configurator: 
	-rm -f configurator
	-docker rmi ${DOCKER_IMAGE_REPO}:${DOCKER_IMAGE_TAG}

deploy-configurator:
	-kubectl create ns configurator		
	-kubectl apply -f deploy/configurator-serviceaccount.yaml
	-kubectl apply -f deploy/configurator-clusterrole.yaml
	-kubectl apply -f deploy/configurator-clusterrolebinding.yaml
	-kubectl apply -f deploy/configurator-deployment.yaml

deploy-crds:
	-kubectl apply -f deploy/crd-customConfigMap.yaml
	-kubectl apply -f deploy/crd-customSecret.yaml

remove-configurator:
	-kubectl delete -f deploy/configurator-deployment.yaml
	-kubectl delete -f deploy/configurator-clusterrolebinding.yaml
	-kubectl delete -f deploy/configurator-clusterrole.yaml
	-kubectl delete -f deploy/configurator-serviceaccount.yaml

cleanup-crds:
	-kubectl delete -f deploy/crd-customConfigMap.yaml
	-kubectl delete -f deploy/crd-customSecret.yaml

build-configurator:
	-go mod vendor
	-go build -o configurator . 
	-sudo docker build . -t ${DOCKER_IMAGE_REPO}:${DOCKER_IMAGE_TAG}

push-image:
	-docker push ${DOCKER_IMAGE_REPO}:${DOCKER_IMAGE_TAG}

helm:
	cd helm && helm package ../helm-src/configurator
	cd helm && helm repo index .

helm-install:
	-helm upgrade --install --create-namespace --namespace configurator configurator helm-src/configurator

helm-uninstall:
	-helm uninstall -n configurator configurator
