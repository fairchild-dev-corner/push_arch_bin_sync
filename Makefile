.PHONY: run run-all stop-all build

run: build
	@bin/push_arch_bin_sync

build:
	@go build -o bin/push_arch_bin_sync cmd/main.go

# Starts the daemon, Prometheus, and Grafana together. Prometheus/Grafana
# output goes to monitoring/logs/ (they're noisy); the daemon's own output
# stays on this terminal since that's what you're usually watching.
# Ctrl+C stops all three - reliably on Linux/macOS (real POSIX signals).
# On native Windows this was tested and confirmed UNRELIABLE: Git-Bash/MSYS
# does not reliably forward Ctrl+C to native .exe background children, so
# processes can be left running after Ctrl+C. On Windows, follow up with
# `make stop-all` (or run monitoring/stop-all.ps1 directly) to force-stop
# anything left over.
run-all: build
	@mkdir -p monitoring/logs
	@bash -c '\
		trap "echo Stopping...; kill 0 2>/dev/null" EXIT INT TERM; \
		echo "Starting Prometheus (log: monitoring/logs/prometheus.log)..."; \
		./monitoring/start-prometheus.sh > monitoring/logs/prometheus.log 2>&1 & \
		ready=0; \
		for i in $$(seq 1 15); do \
			code=$$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9090/-/ready 2>/dev/null); \
			if [ "$$code" = "200" ]; then ready=1; break; fi; \
			sleep 1; \
		done; \
		if [ "$$ready" = "1" ]; then \
			echo "Prometheus is running at http://localhost:9090"; \
		else \
			echo "WARNING: Prometheus did not become ready within 15s - check monitoring/logs/prometheus.log"; \
		fi; \
		echo "Starting Loki (log: monitoring/logs/loki.log)..."; \
		./monitoring/start-loki.sh > monitoring/logs/loki.log 2>&1 & \
		ready=0; \
		for i in $$(seq 1 15); do \
			code=$$(curl -s -o /dev/null -w "%{http_code}" http://localhost:3100/ready 2>/dev/null); \
			if [ "$$code" = "200" ]; then ready=1; break; fi; \
			sleep 1; \
		done; \
		if [ "$$ready" = "1" ]; then \
			echo "Loki is running at http://localhost:3100"; \
		else \
			echo "WARNING: Loki did not become ready within 15s - check monitoring/logs/loki.log"; \
		fi; \
		echo "Starting Grafana (log: monitoring/logs/grafana.log)..."; \
		./monitoring/start-grafana.sh > monitoring/logs/grafana.log 2>&1 & \
		sleep 2; \
		echo "Starting push_arch_bin_sync..."; \
		bin/push_arch_bin_sync & \
		wait \
	'

# Windows-only: force-stops the daemon/Prometheus/Grafana by process name.
# Use this after Ctrl+C on run-all if anything got left running (see the
# comment above run-all for why that can happen on Windows).
stop-all:
	@powershell.exe -NoProfile -ExecutionPolicy Bypass -File monitoring/stop-all.ps1
