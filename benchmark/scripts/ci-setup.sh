#!/usr/bin/env bash
#
# ci-setup.sh — Setup script for benchmark CI environment
#
# This script initializes the database schema and creates a test tenant
# for running benchmarks locally with Docker Compose.
#
# Usage:
#   docker compose -f docker-compose.benchmark.yml up -d
#   ./benchmark/scripts/ci-setup.sh
#   export MNEMO_API_TOKEN=$(cat .benchmark-token)
#   make bench-perf

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Configuration
MYSQL_HOST="${MYSQL_HOST:-localhost}"
MYSQL_PORT="${MYSQL_PORT:-13306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-benchmark}"
MYSQL_DATABASE="${MYSQL_DATABASE:-memorix}"
MEMORIX_URL="${MEMORIX_URL:-http://localhost:18081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
fail()  { echo -e "${RED}[FAIL]${NC} $*" >&2; exit 1; }

# Check if mysql client is available
if command -v mysql &>/dev/null; then
    MYSQL_CMD="mysql"
elif command -v docker &>/dev/null; then
    MYSQL_CMD="docker compose -f $ROOT_DIR/docker-compose.benchmark.yml exec -T mysql mysql"
else
    fail "Neither mysql client nor docker compose is available"
fi

run_mysql() {
    if [ "$MYSQL_CMD" = "mysql" ]; then
        mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" "$@" "$MYSQL_DATABASE"
    else
        docker compose -f "$ROOT_DIR/docker-compose.benchmark.yml" exec -T mysql mysql -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" "$@" "$MYSQL_DATABASE"
    fi
}

info "Setting up benchmark environment..."

# Wait for MySQL
info "Waiting for MySQL to be ready..."
for i in {1..30}; do
    if run_mysql -e "SELECT 1" &>/dev/null; then
        ok "MySQL is ready"
        break
    fi
    sleep 2
done

# Wait for memorix-server
info "Waiting for memorix-server to be ready..."
for i in {1..30}; do
    if curl -sf "$MEMORIX_URL/healthz" >/dev/null 2>&1; then
        ok "memorix-server is ready"
        break
    fi
    sleep 2
done

# Create control plane tables
info "Creating control plane tables..."
run_mysql <<'EOF'
CREATE TABLE IF NOT EXISTS tenants (
  id              VARCHAR(36)   PRIMARY KEY,
  name            VARCHAR(255)  NOT NULL,
  db_host         VARCHAR(255)  NOT NULL,
  db_port         INT           NOT NULL,
  db_user         VARCHAR(255)  NOT NULL,
  db_password     VARCHAR(255)  NOT NULL,
  db_name         VARCHAR(255)  NOT NULL,
  db_tls          TINYINT(1)    NOT NULL DEFAULT 0,
  provider        VARCHAR(50)   NOT NULL,
  cluster_id      VARCHAR(255)  NULL,
  claim_url       TEXT          NULL,
  claim_expires_at TIMESTAMP    NULL,
  status          VARCHAR(20)   NOT NULL DEFAULT 'active',
  schema_version  INT           NOT NULL DEFAULT 1,
  created_at      TIMESTAMP     DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMP     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at      TIMESTAMP     NULL,
  UNIQUE INDEX idx_tenant_name (name),
  INDEX idx_tenant_status (status),
  INDEX idx_tenant_provider (provider)
);

CREATE TABLE IF NOT EXISTS tenant_tokens (
  api_token     VARCHAR(64)   PRIMARY KEY,
  tenant_id     VARCHAR(36)   NOT NULL,
  created_at    TIMESTAMP     DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_tenant (tenant_id)
);
EOF
ok "Control plane tables created"

# Create tenant
TENANT_ID="bench-$(date +%s)"
API_TOKEN="mnemo_bench_$(openssl rand -hex 16)"

info "Creating benchmark tenant: $TENANT_ID"

# Use the memorix user credentials for tenant DB connection
run_mysql <<EOF
INSERT INTO tenants (id, name, db_host, db_port, db_user, db_password, db_name, provider, status, schema_version)
VALUES ('$TENANT_ID', '$TENANT_ID', 'mysql', 3306, 'memorix', 'memorix', 'memorix', 'local', 'active', 1);

INSERT INTO tenant_tokens (api_token, tenant_id)
VALUES ('$API_TOKEN', '$TENANT_ID');
EOF

# Create tenant schema
info "Creating tenant schema..."
run_mysql <<'EOSQL'
CREATE TABLE IF NOT EXISTS memories (
  id              VARCHAR(36)     PRIMARY KEY,
  content         TEXT            NOT NULL,
  source          VARCHAR(100),
  tags            JSON,
  metadata        JSON,
  embedding       TEXT            NULL,
  memory_type     VARCHAR(20)     NOT NULL DEFAULT 'pinned',
  agent_id        VARCHAR(100)    NULL,
  session_id      VARCHAR(100)    NULL,
  state           VARCHAR(20)     NOT NULL DEFAULT 'active',
  version         INT             DEFAULT 1,
  updated_by      VARCHAR(100),
  superseded_by   VARCHAR(36)     NULL,
  created_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  confidence      DECIMAL(3,2)    NOT NULL DEFAULT 1.0,
  access_count    INT             NOT NULL DEFAULT 0,
  last_accessed_at TIMESTAMP     NULL,
  importance_score DECIMAL(5,4)   NULL,
  INDEX idx_memory_type (memory_type),
  INDEX idx_source (source),
  INDEX idx_state (state),
  INDEX idx_agent (agent_id),
  INDEX idx_session (session_id),
  INDEX idx_updated (updated_at),
  INDEX idx_gc_stale (state, last_accessed_at),
  INDEX idx_gc_importance (importance_score)
);

CREATE TABLE IF NOT EXISTS user_profile_facts (
  fact_id           VARCHAR(36)     PRIMARY KEY,
  user_id           VARCHAR(100)    NOT NULL,
  category          VARCHAR(20)     NOT NULL COMMENT 'personal|preference|goal|skill',
  `key`             VARCHAR(100)    NOT NULL,
  value             TEXT            NOT NULL,
  source            VARCHAR(20)     NOT NULL COMMENT 'explicit|inferred',
  confidence        DECIMAL(3,2)    NOT NULL DEFAULT 1.0,
  created_at        TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  updated_at        TIMESTAMP       DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  last_accessed_at  TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_user_facts (user_id),
  INDEX idx_user_category (user_id, category),
  INDEX idx_user_key (user_id, `key`),
  INDEX idx_accessed_confidence (user_id, last_accessed_at, confidence)
);

CREATE TABLE IF NOT EXISTS reconcile_audit_logs (
  log_id            VARCHAR(36)     PRIMARY KEY,
  user_id           VARCHAR(100)    NOT NULL,
  fact_id           VARCHAR(36)     NOT NULL,
  category          VARCHAR(20)     NOT NULL,
  `key`             VARCHAR(100)    NOT NULL,
  old_value         TEXT            NULL,
  new_value         TEXT            NOT NULL,
  decision          VARCHAR(20)     NOT NULL,
  reason            TEXT            NULL,
  source            VARCHAR(100)    NULL,
  created_at        TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_user_audit (user_id, created_at DESC),
  INDEX idx_fact_audit (fact_id, created_at DESC),
  INDEX idx_category_audit (user_id, category, created_at DESC)
);

CREATE TABLE IF NOT EXISTS conversation_summaries (
  id              VARCHAR(36)     PRIMARY KEY,
  user_id         VARCHAR(100)    NOT NULL,
  agent_id        VARCHAR(100)    NOT NULL,
  session_id      VARCHAR(100)    NOT NULL,
  summary         TEXT            NOT NULL,
  token_count     INT             NOT NULL DEFAULT 0,
  created_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_user_session (user_id, session_id),
  INDEX idx_user_agent (user_id, agent_id),
  INDEX idx_created (created_at DESC)
);

CREATE TABLE IF NOT EXISTS memory_gc_logs (
  log_id          VARCHAR(36)     PRIMARY KEY,
  memory_id       VARCHAR(36)     NOT NULL,
  action          VARCHAR(20)     NOT NULL,
  reason          TEXT            NULL,
  metrics_before  JSON            NULL,
  metrics_after   JSON            NULL,
  created_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_memory (memory_id),
  INDEX idx_created (created_at DESC)
);

CREATE TABLE IF NOT EXISTS memory_gc_snapshots (
  snapshot_id     VARCHAR(36)     PRIMARY KEY,
  memory_id       VARCHAR(36)     NOT NULL,
  content         TEXT            NOT NULL,
  tags            JSON            NULL,
  metadata        JSON            NULL,
  snapshot_reason VARCHAR(50)     NOT NULL,
  created_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_memory (memory_id),
  INDEX idx_created (created_at DESC)
);
EOSQL

# Save token to file
echo "$API_TOKEN" > "$ROOT_DIR/.benchmark-token"

ok "Benchmark environment ready!"
echo ""
echo "============================================================"
echo "  Benchmark Environment Setup Complete"
echo "============================================================"
echo ""
echo "  Tenant ID:  $TENANT_ID"
echo "  API Token:  $API_TOKEN"
echo ""
echo "  Token saved to: $ROOT_DIR/.benchmark-token"
echo ""
echo "  To run benchmarks:"
echo "    export MNEMO_API_TOKEN=\$(cat $ROOT_DIR/.benchmark-token)"
echo "    export MNEMO_API_URL=$MEMORIX_URL"
echo "    make bench-perf"
echo ""
echo "============================================================"
