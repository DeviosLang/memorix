# Memorix Benchmark Makefile
#
# Usage:
#   make bench            - Run benchmark (requires MNEMO_API_TOKEN and BENCH_PROMPT_FILE)
#   make bench-report     - Generate report from latest results
#   make bench-clean      - Clean benchmark results
#   make bench-perf       - Run performance benchmark
#   make bench-perf-report - Show latest performance results
#   make bench-perf-clean - Clean performance benchmark results

.PHONY: bench bench-report bench-clean bench-perf bench-perf-report bench-perf-clean help

SCRIPTS_DIR := $(dir $(lastword $(MAKEFILE_LIST)))scripts
RESULTS_DIR := $(dir $(lastword $(MAKEFILE_LIST)))results
PERF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))perf

help:
	@echo "Memorix Benchmark Commands:"
	@echo ""
	@echo "  make bench           Run benchmark (requires MNEMO_API_TOKEN and BENCH_PROMPT_FILE)"
	@echo "  make bench-report    Generate HTML report from latest results"
	@echo "  make bench-clean     Clean benchmark results directory"
	@echo ""
	@echo "  make bench-perf      Run performance benchmark"
	@echo "  make bench-perf-report Show latest performance results"
	@echo "  make bench-perf-clean Clean performance benchmark results"
	@echo ""
	@echo "Required environment variables:"
	@echo "  MNEMO_API_TOKEN      User API token for authentication"
	@echo "  BENCH_PROMPT_FILE    Path to YAML prompt file (for 'bench')"
	@echo ""
	@echo "Optional environment variables:"
	@echo "  MNEMO_API_URL        Server URL (default: http://127.0.0.1:18081)"
	@echo "  BENCH_PROMPT_TIMEOUT Per-prompt timeout in seconds (default: 600)"
	@echo "  BENCH_CONCURRENCY    Number of concurrent sessions (default: 1)"
	@echo ""
	@echo "Performance benchmark variables:"
	@echo "  BENCH_PERF_SCENARIO  Scenario name (default: crud_baseline)"
	@echo "  BENCH_PERF_DURATION  Test duration in seconds (default: 60)"
	@echo "  BENCH_PERF_CONCURRENCY Number of concurrent workers (default: 10)"
	@echo "  BENCH_PERF_RAMP_UP   Ramp-up period in seconds (default: 0)"

bench:
ifndef MNEMO_API_TOKEN
	$(error MNEMO_API_TOKEN is required. export MNEMO_API_TOKEN='mnemo_...')
endif
ifndef BENCH_PROMPT_FILE
	$(error BENCH_PROMPT_FILE is required. export BENCH_PROMPT_FILE='benchmark/prompts/example.yaml')
endif
	bash $(SCRIPTS_DIR)/benchmark.sh

bench-report:
	@latest=$$(ls -td $(RESULTS_DIR)/*/ 2>/dev/null | head -1); \
	if [ -z "$$latest" ]; then \
		echo "ERROR: No benchmark results found."; \
		exit 1; \
	fi; \
	python3 $(SCRIPTS_DIR)/report.py "$$latest/benchmark-results.json" > "$$latest/report.html"; \
	echo "Report written to $$latest/report.html"

bench-clean:
	rm -rf $(RESULTS_DIR)/*
	@mkdir -p $(RESULTS_DIR)
	@touch $(RESULTS_DIR)/.gitkeep
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
	scenario_file=$(PERF_DIR)/scenarios/$${scenario}.yaml; \
	if [ ! -f "$$scenario_file" ]; then \
		echo "ERROR: Scenario file not found: $$scenario_file"; \
		exit 1; \
	fi; \
	echo "Running performance benchmark: $$scenario"; \
	echo "  Duration: $${duration}s | Concurrency: $${concurrency} | Ramp-up: $${ramp_up}s"; \
	python3 $(PERF_DIR)/load_test.py \
		--api-url "$$api_url" \
		--api-token "$${MNEMO_API_TOKEN}" \
		--scenario-file "$$scenario_file" \
		--results-dir $(PERF_DIR)/results \
		--duration $$duration \
		--concurrency $$concurrency \
		--ramp-up $$ramp_up

bench-perf-report:
	@latest=$$(ls -td $(PERF_DIR)/results/perf-*.json 2>/dev/null | head -1); \
	if [ -z "$$latest" ]; then \
		echo "ERROR: No performance benchmark results found."; \
		exit 1; \
	fi; \
	echo "Latest performance results: $$latest"; \
	cat "$$latest"

bench-perf-clean:
	rm -rf $(PERF_DIR)/results/*
	@mkdir -p $(PERF_DIR)/results
	@touch $(PERF_DIR)/results/.gitkeep
	@echo "Performance benchmark results cleaned."
