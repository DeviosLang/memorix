# Memorix Benchmark Makefile
#
# Usage:
#   make bench            - Run benchmark (requires MNEMO_API_TOKEN and BENCH_PROMPT_FILE)
#   make bench-report     - Generate report from latest results
#   make bench-clean      - Clean benchmark results

.PHONY: bench bench-report bench-clean help

SCRIPTS_DIR := $(dir $(lastword $(MAKEFILE_LIST)))scripts
RESULTS_DIR := $(dir $(lastword $(MAKEFILE_LIST)))results

help:
	@echo "Memorix Benchmark Commands:"
	@echo ""
	@echo "  make bench           Run benchmark (requires MNEMO_API_TOKEN and BENCH_PROMPT_FILE)"
	@echo "  make bench-report    Generate HTML report from latest results"
	@echo "  make bench-clean     Clean benchmark results directory"
	@echo ""
	@echo "Required environment variables:"
	@echo "  MNEMO_API_TOKEN      User API token for authentication"
	@echo "  BENCH_PROMPT_FILE    Path to YAML prompt file"
	@echo ""
	@echo "Optional environment variables:"
	@echo "  MNEMO_API_URL        Server URL (default: http://127.0.0.1:18081)"
	@echo "  BENCH_PROMPT_TIMEOUT Per-prompt timeout in seconds (default: 600)"
	@echo "  BENCH_CONCURRENCY    Number of concurrent sessions (default: 1)"

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
