-- Control plane schema (MNEMO_DSN).

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
  status          VARCHAR(20)   NOT NULL DEFAULT 'provisioning'
                  COMMENT 'provisioning|active|suspended|deleted',
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


-- Tenant data plane schema (per-tenant TiDB Serverless).
CREATE TABLE IF NOT EXISTS memories (
  id              VARCHAR(36)     PRIMARY KEY,
  content         MEDIUMTEXT      NOT NULL,
  source          VARCHAR(100),
  tags            JSON,
  metadata        JSON,
  embedding       VECTOR(1536)    NULL,

  -- Classification
  memory_type     VARCHAR(20)     NOT NULL DEFAULT 'pinned'
                  COMMENT 'pinned|insight|digest',

  -- Agent & session tracking
  agent_id        VARCHAR(100)    NULL     COMMENT 'Agent that created this memory',
  session_id      VARCHAR(100)    NULL     COMMENT 'Session this memory originated from',

  -- Lifecycle
  state           VARCHAR(20)     NOT NULL DEFAULT 'active'
                  COMMENT 'active|paused|archived|deleted|stale',
  version         INT             DEFAULT 1,
  updated_by      VARCHAR(100),
  created_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  superseded_by   VARCHAR(36)     NULL     COMMENT 'ID of the memory that replaced this one',

  -- GC (Memory Garbage Collection) fields
  confidence      DECIMAL(3,2)    NOT NULL DEFAULT 1.0
                  COMMENT '0.00-1.00, confidence score for inferred memories',
  access_count    INT             NOT NULL DEFAULT 0
                  COMMENT 'Number of times this memory was accessed',
  last_accessed_at TIMESTAMP      NULL
                  COMMENT 'Last time this memory was accessed/retrieved',
  importance_score DECIMAL(5,4)   NULL
                  COMMENT 'Computed importance score (0.0000-1.0000)',

  INDEX idx_memory_type         (memory_type),
  INDEX idx_source              (source),
  INDEX idx_state               (state),
  INDEX idx_agent               (agent_id),
  INDEX idx_session             (session_id),
  INDEX idx_updated             (updated_at),
  INDEX idx_gc_stale            (state, last_accessed_at),
  INDEX idx_gc_importance       (importance_score)
);

-- Full-text search index (TiDB Cloud Serverless with MULTILINGUAL tokenizer).
-- ADD_COLUMNAR_REPLICA_ON_DEMAND auto-provisions TiFlash on Serverless clusters.
-- Run after the memories table is created. Safe to re-run (fails silently if index exists).
-- ALTER TABLE memories
--   ADD FULLTEXT INDEX idx_fts_content (content)
--   WITH PARSER MULTILINGUAL
--   ADD_COLUMNAR_REPLICA_ON_DEMAND;

-- Vector index requires TiFlash. May fail on plain MySQL; safe to ignore.
-- ALTER TABLE memories ADD VECTOR INDEX idx_cosine ((VEC_COSINE_DISTANCE(embedding)));

-- Auto-embedding variant (TiDB Cloud Serverless only):
-- Replace the embedding column above with a generated column:
--
--   embedding VECTOR(1024) GENERATED ALWAYS AS (
--     EMBED_TEXT("tidbcloud_free/amazon/titan-embed-text-v2", content)
--   ) STORED,
--
-- Then add vector index:
--   VECTOR INDEX idx_cosine ((VEC_COSINE_DISTANCE(embedding)))
--
-- Set MNEMO_EMBED_AUTO_MODEL=tidbcloud_free/amazon/titan-embed-text-v2 to enable.


-- Migration: tombstone -> state (4-step plan).
-- Step 1: Add new columns (backward compatible — existing code still uses tombstone).
-- ALTER TABLE memories
--   ADD COLUMN memory_type  VARCHAR(20) NOT NULL DEFAULT 'pinned',
--   ADD COLUMN agent_id     VARCHAR(100) NULL,
--   ADD COLUMN session_id   VARCHAR(100) NULL,
--   ADD COLUMN state        VARCHAR(20) NOT NULL DEFAULT 'active',
--   ADD COLUMN superseded_by VARCHAR(36) NULL;
-- CREATE INDEX idx_memory_type ON memories(memory_type);
-- CREATE INDEX idx_state ON memories(state);
-- CREATE INDEX idx_agent ON memories(agent_id);
-- CREATE INDEX idx_session ON memories(session_id);
-- Step 2: Migrate tombstoned records.
-- UPDATE memories SET state = 'deleted', deleted_at = updated_at WHERE tombstone = 1;
-- Step 3: Add constraint (AFTER code migration).
-- ALTER TABLE memories ADD CONSTRAINT chk_state CHECK (state IN ('active','paused','archived','deleted'));
-- Step 4: Drop tombstone (separate deployment).
-- ALTER TABLE memories DROP COLUMN tombstone;
-- DROP INDEX idx_tombstone ON memories;

-- Upload task tracking (control plane).
CREATE TABLE IF NOT EXISTS upload_tasks (
  task_id       VARCHAR(36)   PRIMARY KEY,
  tenant_id     VARCHAR(36)   NOT NULL,
  file_name     VARCHAR(255)  NOT NULL,
  file_path     TEXT          NOT NULL,
  agent_id      VARCHAR(100)  NULL,
  session_id    VARCHAR(100)  NULL,
  file_type     VARCHAR(20)   NOT NULL COMMENT 'session|memory',
  total_chunks  INT           NOT NULL DEFAULT 0,
  done_chunks   INT           NOT NULL DEFAULT 0,
  status        VARCHAR(20)   NOT NULL DEFAULT 'pending'
                COMMENT 'pending|processing|done|failed',
  error_msg     TEXT          NULL,
  created_at    TIMESTAMP     DEFAULT CURRENT_TIMESTAMP,
  updated_at    TIMESTAMP     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_upload_tenant (tenant_id),
  INDEX idx_upload_poll (status, created_at)
);

-- User profile facts (structured long-term facts about users).
-- Based on ChatGPT's "third layer" user memory - structured profile storage.
CREATE TABLE IF NOT EXISTS user_profile_facts (
  fact_id           VARCHAR(36)     PRIMARY KEY,
  user_id           VARCHAR(100)    NOT NULL,
  category          VARCHAR(20)     NOT NULL
                      COMMENT 'personal|preference|goal|skill',
  `key`             VARCHAR(100)    NOT NULL,
  value             TEXT            NOT NULL,
  source            VARCHAR(20)     NOT NULL
                      COMMENT 'explicit|inferred',
  confidence        DECIMAL(3,2)    NOT NULL DEFAULT 1.0
                      COMMENT '0.00-1.00, confidence for inferred facts',
  created_at        TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  updated_at        TIMESTAMP       DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  last_accessed_at  TIMESTAMP       DEFAULT CURRENT_TIMESTAMP
                      COMMENT 'Used for capacity-based cleanup',
  INDEX idx_user_facts (user_id),
  INDEX idx_user_category (user_id, category),
  INDEX idx_user_key (user_id, `key`),
  INDEX idx_accessed_confidence (user_id, last_accessed_at, confidence)
);

-- Reconciliation audit logs (tracks LLM-driven memory conflict resolution).
-- Every reconciliation decision is logged for traceability and debugging.
CREATE TABLE IF NOT EXISTS reconcile_audit_logs (
  log_id            VARCHAR(36)     PRIMARY KEY,
  user_id           VARCHAR(100)    NOT NULL,
  fact_id           VARCHAR(36)     NOT NULL
                      COMMENT 'The fact ID that was reconciled',
  category          VARCHAR(20)     NOT NULL
                      COMMENT 'personal|preference|goal|skill',
  `key`             VARCHAR(100)    NOT NULL,
  old_value         TEXT            NULL
                      COMMENT 'Previous value (empty for new facts)',
  new_value         TEXT            NOT NULL
                      COMMENT 'Incoming value',
  decision          VARCHAR(20)     NOT NULL
                      COMMENT 'UPDATE|APPEND|IGNORE',
  reason            TEXT            NULL
                      COMMENT 'LLM explanation for the decision',
  source            VARCHAR(100)    NULL
                      COMMENT 'Agent name that provided the new fact',
  created_at        TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_user_audit (user_id, created_at DESC),
  INDEX idx_fact_audit (fact_id, created_at DESC),
  INDEX idx_category_audit (user_id, category, created_at DESC)
);

-- Conversation summaries (recent conversation summary layer).
-- Based on ChatGPT's "fourth layer" - pre-computed summaries of recent conversations.
-- Advantages: zero retrieval latency, provides quick context for ongoing conversations.
CREATE TABLE IF NOT EXISTS conversation_summaries (
  summary_id     VARCHAR(36)     PRIMARY KEY,
  user_id        VARCHAR(100)    NOT NULL,
  session_id     VARCHAR(100)    NULL
                 COMMENT 'Session ID this summary originated from',
  title          VARCHAR(200)    NOT NULL
                 COMMENT 'LLM-generated conversation title',
  summary        VARCHAR(500)    NOT NULL
                 COMMENT 'Summary within 200 Chinese characters (~500 bytes)',
  key_topics     JSON            NULL
                 COMMENT 'Array of key topic tags',
  user_intent    VARCHAR(300)    NOT NULL
                 COMMENT 'Core user intent from conversation',
  created_at     TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_user_summaries (user_id, created_at DESC),
  INDEX idx_session (session_id)
) COMMENT 'Recent conversation summaries - sliding window of 15-20 per user';

-- Memory GC audit logs (tracks memory garbage collection operations).
-- Every GC operation is logged for traceability and recovery purposes.
CREATE TABLE IF NOT EXISTS memory_gc_logs (
  log_id           VARCHAR(36)     PRIMARY KEY,
  memory_id        VARCHAR(36)     NOT NULL,
  tenant_id        VARCHAR(36)     NOT NULL,
  content_preview  VARCHAR(500)    NULL
                   COMMENT 'First 500 chars of deleted content for recovery',
  source           VARCHAR(100)    NULL,
  memory_type      VARCHAR(20)     NULL,
  state            VARCHAR(20)     NOT NULL
                   COMMENT 'State before deletion',
  confidence       DECIMAL(3,2)    NULL,
  access_count     INT             NULL,
  last_accessed_at TIMESTAMP       NULL,
  importance_score DECIMAL(5,4)    NULL,
  deletion_reason  VARCHAR(50)     NOT NULL
                   COMMENT 'stale|low_importance|capacity|manual',
  gc_run_id        VARCHAR(36)     NOT NULL
                   COMMENT 'Groups logs from the same GC run',
  created_at       TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_gc_tenant (tenant_id, created_at DESC),
  INDEX idx_gc_run (gc_run_id),
  INDEX idx_gc_memory (memory_id)
) COMMENT 'Memory GC audit logs for traceability and recovery';

-- Memory GC snapshots (pre-deletion backup for recovery).
-- Stores full memory data before bulk deletion for potential recovery.
CREATE TABLE IF NOT EXISTS memory_gc_snapshots (
  snapshot_id     VARCHAR(36)     PRIMARY KEY,
  gc_run_id       VARCHAR(36)     NOT NULL,
  tenant_id       VARCHAR(36)     NOT NULL,
  memory_count    INT             NOT NULL
                  COMMENT 'Number of memories in this snapshot',
  snapshot_data   MEDIUMTEXT      NOT NULL
                  COMMENT 'JSON array of memory objects',
  created_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  expires_at      TIMESTAMP       NOT NULL
                  COMMENT 'When this snapshot can be purged',
  INDEX idx_gc_snapshot_tenant (tenant_id, created_at DESC),
  INDEX idx_gc_snapshot_run (gc_run_id),
  INDEX idx_gc_snapshot_expires (expires_at)
) COMMENT 'Pre-deletion snapshots for memory recovery';
