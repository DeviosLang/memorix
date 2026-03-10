MAKEFILE_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
IMG ?= $(REGISTRY)/memorix-server:$(COMMIT)

.PHONY: build build-linux vet clean run test test-integration docker docker-build docker-run

build:
	mkdir -p $(MAKEFILE_DIR)/./bin
	cd server && CGO_ENABLED=0 go build -o ./bin/memorix-server ./cmd/memorix-server


build-linux:
	mkdir -p $(MAKEFILE_DIR)/./bin
	cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/memorix-server ./cmd/memorix-server

vet:
	cd server && go vet ./...

test:
	cd server && go test -race -count=1 ./...

test-integration:
	cd server && go test -tags=integration -race -count=1 -v ./internal/repository/tidb/
clean:
	rm -f server/memorix-server

run: build
	cd server && MNEMO_DSN="$(MNEMO_DSN)" ./memorix-server

docker: build-linux
	docker build --platform=linux/amd64 -q -f ./server/Dockerfile -t $(IMG) .

docker-build:
	docker build -t memorix-server ./server

docker-run: docker-build
	-docker stop memorix-server 2>/dev/null; docker rm memorix-server 2>/dev/null; true
	docker run -d --name memorix-server -p 8080:8080 \
		-e TZ=Asia/Shanghai \
		-v /etc/localtime:/etc/localtime:ro \
		-e MNEMO_DSN="$(MNEMO_DSN)" \
		$(if $(MNEMO_LLM_API_KEY),-e MNEMO_LLM_API_KEY="$(MNEMO_LLM_API_KEY)") \
		$(if $(MNEMO_LLM_BASE_URL),-e MNEMO_LLM_BASE_URL="$(MNEMO_LLM_BASE_URL)") \
		$(if $(MNEMO_LLM_MODEL),-e MNEMO_LLM_MODEL="$(MNEMO_LLM_MODEL)") \
		memorix-server

