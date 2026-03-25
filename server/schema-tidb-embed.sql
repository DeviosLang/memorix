-- ============================================================
-- Control plane schema（和 schema.sql 相同，建在 MNEMO_DSN 指向的库）
-- ============================================================

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

-- ============================================================
-- Tenant data plane schema
-- 关键区别：embedding 改为 TiDB EMBED_TEXT() 自动生成列（1024维）
-- 需要 TiDB Serverless，并在 configmap 中设置：
--   MNEMO_EMBED_AUTO_MODEL=tidbcloud_free/amazon/titan-embed-text-v2
--   MNEMO_EMBED_AUTO_DIMS=1024
-- ============================================================

CREATE TABLE IF NOT EXISTS memories (
  id              VARCHAR(36)     PRIMARY KEY,
  content         MEDIUMTEXT      NOT NULL,
  source          VARCHAR(100),
  tags            JSON,
  metadata        JSON,

  -- TiDB 自动 embedding：由 EMBED_TEXT() 在写入时自动生成，无需客户端计算
  embedding       VECTOR(1024) GENERATED ALWAYS AS (
    EMBED_TEXT("tidbcloud_free/amazon/titan-embed-text-v2", content)
  ) STORED,

  memory_type     VARCHAR(20)     NOT NULL DEFAULT 'pinned'
                  COMMENT 'pinned|insight|digest',
  agent_id        VARCHAR(100)    NULL,
  session_id      VARCHAR(100)    NULL,
  state           VARCHAR(20)     NOT NULL DEFAULT 'active'
                  COMMENT 'active|paused|archived|deleted|stale',
  version         INT             DEFAULT 1,
  updated_by      VARCHAR(100),
  created_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  superseded_by   VARCHAR(36)     NULL,
  confidence      DECIMAL(3,2)    NOT NULL DEFAULT 1.0,
  access_count    INT             NOT NULL DEFAULT 0,
  last_accessed_at TIMESTAMP      NULL,
  importance_score DECIMAL(5,4)   NULL,

  INDEX idx_memory_type   (memory_type),
  INDEX idx_source        (source),
  INDEX idx_state         (state),
  INDEX idx_agent         (agent_id),
  INDEX idx_session       (session_id),
  INDEX idx_updated       (updated_at),
  INDEX idx_gc_stale      (state, last_accessed_at),
  INDEX idx_gc_importance (importance_score)
);

-- 向量索引（需要 TiFlash，TiDB Serverless 自动支持）
-- embedding 列是生成列，VEC_COSINE_DISTANCE 必须和 ORDER BY 中写法一致
ALTER TABLE memories
  ADD VECTOR INDEX idx_vec_cosine ((VEC_COSINE_DISTANCE(embedding)));

-- ============================================================
-- 其余表（与 schema.sql 相同）
-- ============================================================

CREATE TABLE IF NOT EXISTS user_profile_facts (
  fact_id           VARCHAR(36)     PRIMARY KEY,
  user_id           VARCHAR(100)    NOT NULL,
  category          VARCHAR(20)     NOT NULL,
  `key`             VARCHAR(100)    NOT NULL,
  value             TEXT            NOT NULL,
  source            VARCHAR(20)     NOT NULL,
  confidence        DECIMAL(3,2)    NOT NULL DEFAULT 1.0,
  created_at        TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  updated_at        TIMESTAMP       DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  last_accessed_at  TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_user_facts     (user_id),
  INDEX idx_user_category  (user_id, category),
  INDEX idx_user_key       (user_id, `key`),
  INDEX idx_accessed_confidence (user_id, last_accessed_at, confidence)
);

CREATE TABLE IF NOT EXISTS reconcile_audit_logs (
  log_id     VARCHAR(36)  PRIMARY KEY,
  user_id    VARCHAR(100) NOT NULL,
  fact_id    VARCHAR(36)  NOT NULL,
  category   VARCHAR(20)  NOT NULL,
  `key`      VARCHAR(100) NOT NULL,
  old_value  TEXT         NULL,
  new_value  TEXT         NOT NULL,
  decision   VARCHAR(20)  NOT NULL,
  reason     TEXT         NULL,
  source     VARCHAR(100) NULL,
  created_at TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_user_audit     (user_id, created_at DESC),
  INDEX idx_fact_audit     (fact_id, created_at DESC),
  INDEX idx_category_audit (user_id, category, created_at DESC)
);

CREATE TABLE IF NOT EXISTS conversation_summaries (
  summary_id  VARCHAR(36)  PRIMARY KEY,
  user_id     VARCHAR(100) NOT NULL,
  session_id  VARCHAR(100) NULL,
  title       VARCHAR(200) NOT NULL,
  summary     VARCHAR(500) NOT NULL,
  key_topics  JSON         NULL,
  user_intent VARCHAR(300) NOT NULL,
  created_at  TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_user_summaries (user_id, created_at DESC),
  INDEX idx_session        (session_id)
) COMMENT 'Recent conversation summaries - sliding window of 15-20 per user';

CREATE TABLE IF NOT EXISTS memory_gc_logs (
  log_id           VARCHAR(36)   PRIMARY KEY,
  memory_id        VARCHAR(36)   NOT NULL,
  tenant_id        VARCHAR(36)   NOT NULL,
  content_preview  VARCHAR(500)  NULL,
  source           VARCHAR(100)  NULL,
  memory_type      VARCHAR(20)   NULL,
  state            VARCHAR(20)   NOT NULL,
  confidence       DECIMAL(3,2)  NULL,
  access_count     INT           NULL,
  last_accessed_at TIMESTAMP     NULL,
  importance_score DECIMAL(5,4)  NULL,
  deletion_reason  VARCHAR(50)   NOT NULL,
  gc_run_id        VARCHAR(36)   NOT NULL,
  created_at       TIMESTAMP     DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_gc_tenant (tenant_id, created_at DESC),
  INDEX idx_gc_run    (gc_run_id),
  INDEX idx_gc_memory (memory_id)
);

CREATE TABLE IF NOT EXISTS memory_gc_snapshots (
  snapshot_id   VARCHAR(36)  PRIMARY KEY,
  gc_run_id     VARCHAR(36)  NOT NULL,
  tenant_id     VARCHAR(36)  NOT NULL,
  memory_count  INT          NOT NULL,
  snapshot_data MEDIUMTEXT   NOT NULL,
  created_at    TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,
  expires_at    TIMESTAMP    NOT NULL,
  INDEX idx_gc_snapshot_tenant  (tenant_id, created_at DESC),
  INDEX idx_gc_snapshot_run     (gc_run_id),
  INDEX idx_gc_snapshot_expires (expires_at)
);
