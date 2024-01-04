HOSTNAME := $(shell hostname)
BINARY_NAME := sre-server
BUILD_FLAGS=-o ./bin/${BINARY_NAME}
CERTS := "./certs"
DOCKER_IMAGE := mrkooll-sre-server

.DEFAULT_GOAL := cluster

.PHONY: clean

dependencies:
	@echo "Checking vendor"
	@if [ ! -d vendor ]; then \
        go mod vendor; \
	fi

build: dependencies
	@echo "building SRE server binary for Linux"
	GOOS=linux go build ${BUILD_FLAGS} ./sre-server/main.go

build-local: dependencies
	@echo "building SRE server binary for this host"
	go build ${BUILD_FLAGS} ./sre-server/main.go

clean:
	@echo "Full cleanup"
	go clean
	rm -rf bin
	rm -f extensions.cnf
	rm -rf ${CERTS}
	rm -f helm/sre-server-*.tgz
	docker image rm ${DOCKER_IMAGE}:testing
	kind delete cluster --name mrkooll

certs-dir:
	@if [ ! -d certs ]; then \
	mkdir certs; \
	fi

extensions.cnf: certs-dir
	@if [ ! -f $@ ]; then \
	echo "[ server_extensions ]" > $@ ;\
	echo "keyUsage = critical, digitalSignature, keyEncipherment" >> $@ ;\
	echo "extendedKeyUsage = serverAuth" >> $@ ;\
	echo "[ client_extensions ]" >> $@ ;\
	echo "keyUsage = critical, digitalSignature" >> $@ ;\
	echo "extendedKeyUsage = clientAuth" >> $@ ;\
	fi

${CERTS}/ca.crt: extensions.cnf
	@echo "Generating CA certificate"
	openssl ecparam -name prime256v1 -genkey -noout -out ${CERTS}/ca.key
	openssl req -x509 -new -nodes -key ${CERTS}/ca.key -sha256 -days 3650 -out ${CERTS}/ca.crt \
	-subj "/C=US/ST=Texas/L=Leander/O=Home/OU=Den/CN=SreRootCA" \
	-addext "keyUsage = critical, cRLSign, keyCertSign" \
	-addext "extendedKeyUsage = serverAuth, clientAuth"

${CERTS}/server.crt: extensions.cnf
	@echo "Generating server certificate"
	openssl ecparam -name prime256v1 -genkey -noout -out ${CERTS}/server.key
	openssl req -new -key ${CERTS}/server.key -out ${CERTS}/server.csr \
	-subj "/C=US/ST=Texas/L=Leander/O=Home/OU=Den/CN=localhost" \
	-addext "keyUsage = critical, digitalSignature, keyEncipherment" \
	-addext "extendedKeyUsage = serverAuth"
	openssl x509 -req -in ${CERTS}/server.csr -CA ${CERTS}/ca.crt -CAkey ${CERTS}/ca.key \
	-CAcreateserial -out ${CERTS}/server.crt -days 365 \
	-extfile extensions.cnf -extensions server_extensions

${CERTS}/client.crt: extensions.cnf
	@echo "Generating client certificate"
	openssl ecparam -name prime256v1 -genkey -noout -out ${CERTS}/client.key
	openssl req -new -key ${CERTS}/client.key -out ${CERTS}/client.csr \
	-subj "/C=US/ST=Texas/L=Leander/O=Home/OU=Den/CN=client" \
	-addext "keyUsage = critical, digitalSignature" \
	-addext "extendedKeyUsage = clientAuth"
	openssl x509 -req -in ${CERTS}/client.csr -CA ${CERTS}/ca.crt -CAkey ${CERTS}/ca.key \
	-CAcreateserial -out ${CERTS}/client.crt -days 365 \
	-extfile extensions.cnf -extensions client_extensions

docker: build
	@echo "Building docker image"
	docker image build -t ${DOCKER_IMAGE}:testing .

helm: docker
	@echo "Building helm chart"
	cd helm && helm package sre-server

cluster: docker ${CERTS}/ca.crt ${CERTS}/server.crt helm
	@echo "Creting and configuring cluster"
	kind create cluster --name mrkooll --config ./yaml/kind-cluster.yml
	kind load docker-image ${DOCKER_IMAGE}:testing --name mrkooll

	kubectl create secret generic sre-tls-secrets \
	--from-file=server-cert.pem=${CERTS}/server.crt \
	--from-file=server-key.pem=${CERTS}/server.key \
	--from-file=ca-cert.pem=${CERTS}/ca.crt
	helm install sre-server-release helm/sre-server

deploymentready:
	kubectl rollout status deployment/sre-server-deployment
	sleep 3

test: ${CERTS}/client.crt deploymentready
	@echo "Running tests"
	./sre-server/test.sh

