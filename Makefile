MAKEFILE_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
IMG ?= $(REGISTRY)/memorix-server:$(COMMIT)

.PHONY: build build-linux vet clean run test test-integration docker docker-build docker-run bench bench-report bench-clean

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
		$(if $(MNEMO_EMBED_API_KEY),-e MNEMO_EMBED_API_KEY="$(MNEMO_EMBED_API_KEY)") \
		$(if $(MNEMO_EMBED_BASE_URL),-e MNEMO_EMBED_BASE_URL="$(MNEMO_EMBED_BASE_URL)") \
		$(if $(MNEMO_EMBED_MODEL),-e MNEMO_EMBED_MODEL="$(MNEMO_EMBED_MODEL)") \
		memorix-server

# Benchmark targets
bench:
ifndef MNEMO_API_TOKEN
	$(error MNEMO_API_TOKEN is required. export MNEMO_API_TOKEN='mnemo_...')
endif
ifndef BENCH_PROMPT_FILE
	$(error BENCH_PROMPT_FILE is required. export BENCH_PROMPT_FILE='benchmark/prompts/example.yaml')
endif
	bash $(MAKEFILE_DIR)/benchmark/scripts/benchmark.sh

bench-report:
	@latest=$$(ls -td $(MAKEFILE_DIR)/benchmark/results/*/ 2>/dev/null | head -1); \
	if [ -z "$$latest" ]; then \
		echo "ERROR: No benchmark results found."; \
		exit 1; \
	fi; \
	python3 $(MAKEFILE_DIR)/benchmark/scripts/report.py "$$latest/benchmark-results.json" > "$$latest/report.html"; \
	echo "Report written to $$latest/report.html"

bench-clean:
	rm -rf $(MAKEFILE_DIR)/benchmark/results/*
	@mkdir -p $(MAKEFILE_DIR)/benchmark/results
	@touch $(MAKEFILE_DIR)/benchmark/results/.gitkeep
	@echo "Benchmark results cleaned."

