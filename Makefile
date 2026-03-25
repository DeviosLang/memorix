MAKEFILE_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
IMG ?= $(REGISTRY)/memorix-server:$(COMMIT)

.PHONY: build build-linux vet clean run test test-integration docker docker-build docker-run bench bench-report bench-clean bench-perf bench-perf-report bench-perf-clean bench-baseline-show bench-baseline-save bench-baseline-compare bench-ci-setup

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

TIMESTAMP := $(shell date +%Y%m%d%H%M%S)
REGISTRY_IMG := mirrors.tencent.com/cvm/memorix

docker-build:
	docker build -t $(REGISTRY_IMG):latest -t $(REGISTRY_IMG):$(TIMESTAMP) ./server
	docker push $(REGISTRY_IMG):latest
	docker push $(REGISTRY_IMG):$(TIMESTAMP)

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
		$(REGISTRY_IMG):latest

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

# Performance benchmark targets
bench-perf:
ifndef MNEMO_API_TOKEN
	$(error MNEMO_API_TOKEN is required. export MNEMO_API_TOKEN='mnemo_...')
endif
	@scenario=$${BENCH_PERF_SCENARIO:-crud_baseline}; \
	duration=$${BENCH_PERF_DURATION:-60}; \
	concurrency=$${BENCH_PERF_CONCURRENCY:-10}; \
	ramp_up=$${BENCH_PERF_RAMP_UP:-0}; \
	api_url=$${MNEMO_API_URL:-http://127.0.0.1:18081}; \
	results_dir=$(MAKEFILE_DIR)/benchmark/perf/results; \
	scenario_file=$(MAKEFILE_DIR)/benchmark/perf/scenarios/$${scenario}.yaml; \
	if [ ! -f "$$scenario_file" ]; then \
		echo "ERROR: Scenario file not found: $$scenario_file"; \
		exit 1; \
	fi; \
	echo "Running performance benchmark: $$scenario"; \
	echo "  Duration: $${duration}s | Concurrency: $${concurrency} | Ramp-up: $${ramp_up}s"; \
	python3 $(MAKEFILE_DIR)/benchmark/perf/load_test.py \
		--api-url "$$api_url" \
		--api-token "$${MNEMO_API_TOKEN}" \
		--scenario-file "$$scenario_file" \
		--results-dir "$$results_dir" \
		--duration $$duration \
		--concurrency $$concurrency \
		--ramp-up $$ramp_up

bench-perf-report:
	@latest=$$(ls -td $(MAKEFILE_DIR)/benchmark/perf/results/perf-*.json 2>/dev/null | head -1); \
	if [ -z "$$latest" ]; then \
		echo "ERROR: No performance benchmark results found."; \
		exit 1; \
	fi; \
	echo "Latest performance results: $$latest"; \
	cat "$$latest"

bench-perf-clean:
	rm -rf $(MAKEFILE_DIR)/benchmark/perf/results/*
	@mkdir -p $(MAKEFILE_DIR)/benchmark/perf/results
	@touch $(MAKEFILE_DIR)/benchmark/perf/results/.gitkeep
	@echo "Performance benchmark results cleaned."

# Baseline management targets
bench-baseline-show:
	@python3 $(MAKEFILE_DIR)/benchmark/scripts/baseline.py show

bench-baseline-save:
	@latest=$$(ls -td $(MAKEFILE_DIR)/benchmark/perf/results/perf-*.json 2>/dev/null | head -1); \
	if [ -z "$$latest" ]; then \
		echo "ERROR: No performance benchmark results found. Run 'make bench-perf' first."; \
		exit 1; \
	fi; \
	echo "Saving $$latest as baseline..."; \
	python3 $(MAKEFILE_DIR)/benchmark/scripts/baseline.py save "$$latest"

bench-baseline-compare:
	@latest=$$(ls -td $(MAKEFILE_DIR)/benchmark/perf/results/perf-*.json 2>/dev/null | head -1); \
	if [ -z "$$latest" ]; then \
		echo "ERROR: No performance benchmark results found. Run 'make bench-perf' first."; \
		exit 1; \
	fi; \
	python3 $(MAKEFILE_DIR)/benchmark/scripts/baseline.py compare "$$latest"

# CI setup target
bench-ci-setup:
	@bash $(MAKEFILE_DIR)/benchmark/scripts/ci-setup.sh

