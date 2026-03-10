# Memory Lifecycle & Migration Plan

## 1. Goal
Add first-class lifecycle management for memories so the system can:
- Separate short-lived context from durable knowledge.
- Promote/demote memories automatically using server-side scheduling.
- Forget safely via archive-first policies (not hard delete).
- Support tenant-level knowledge base migration with minimal downtime.
- Normalize stable patterns into executable `rule` objects.
- Generate actionable skill/MCP recommendations from aggregated memory signals.

This plan keeps all scheduling decisions in `memorix-server` and uses DB lease + optimistic locking to stay correct under distributed plugin deployments.

## 2. Current Gaps
- Memory model has `memory_type` and `state`, but no temporal tier (`working/short/long`).
- No usage telemetry (`hit_count`, `last_access_at`) for salience-based retention.
- No periodic compaction/promotion job.
- No structured cross-tenant migration workflow.
- No first-class `rule` entity for normalized, reusable behavior.
- No recommendation pipeline for proposing new skills/tools from repeated memory patterns.

## 3. Data Model Changes
Add columns to tenant `memories` table:
- `tier VARCHAR(16) NOT NULL DEFAULT 'short'` (`working|short|long|reference`).
- `salience_score DOUBLE NOT NULL DEFAULT 0`.
- `reuse_count BIGINT NOT NULL DEFAULT 0`.
- `last_access_at TIMESTAMP NULL`.
- `decay_anchor_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`.
- `retention_policy VARCHAR(16) NOT NULL DEFAULT 'auto'` (`auto|pin|ttl`).
- `archive_reason VARCHAR(32) NULL`.

Add rule/recommendation tables (tenant data plane):
- `memory_rules(id PK, tenant_id, name, scope, condition_json, action_json, confidence, status, source_memory_ids, created_at, updated_at)`.
- `memory_rule_events(id PK, rule_id, memory_id, event_type, event_at, evidence_json)`.
- `memory_recommendations(id PK, tenant_id, rec_type, title, rationale, evidence_json, impact_score, status, created_at, reviewed_at)`.

Add job control tables in control plane:
- `scheduler_leases(job_name PK, owner_id, lease_until, heartbeat_at, version)`.
- `memory_jobs(id PK, tenant_id, job_type, status, run_at, payload, attempts, updated_at)`.
- `memory_job_runs(id PK, tenant_id, job_type, started_at, finished_at, processed, archived, promoted, demoted, error)`.

## 4. Retrieval & Scoring Updates
On memory retrieval/search hit:
- Increment `reuse_count`.
- Set `last_access_at = NOW()`.
- Recompute `salience_score` asynchronously:
  `score = w1*quality + w2*log1p(reuse_count) + w3*recency_decay + w4*pinned_boost`.

Hybrid ranking stays RRF-based but multiplies by a bounded salience factor, e.g. `final = rrf * (1 + clamp(salience_score, 0, 1))`.

## 5. Forgetting Policy (Archive-First)
Never hard-delete by scheduler.
- If `state=active` and `last_access_at` stale beyond threshold and salience below floor -> set `state='archived'`, record `archive_reason='cold_expired'`.
- Keep archived rows searchable only with explicit filter.
- Add optional purge policy: delete archived rows older than N days only when tenant enables `hard_purge=true`.

This satisfies “long-term no-hit => archive, not brutal cleanup”.

## 6. Promotion/Demotion Worker
### Decision rules
Run periodic evaluation per tenant:
- Promote `short -> long` when reuse and salience exceed thresholds.
- Demote `long -> short` when decay lowers salience below threshold.
- Move very hot, user-critical memories to `reference` only via explicit pin or policy.

### Consistency under distributed deployment
Plugins are stateless and can scale horizontally; they do not run lifecycle jobs.
All job execution is centralized in server workers with:
- DB lease for single active scheduler per job type.
- Optimistic lock on memory row (`WHERE id=? AND version=?`) for state/tier transitions.
- Idempotent job payloads keyed by `(tenant_id, memory_id, decision_window)`.

If a worker crashes, lease expires and another worker resumes safely.

## 7. Rule Normalization (Memory -> Rule Object)
Rule is a result object, not free text advice. It encodes:
- `condition`: when the pattern applies.
- `action`: what system/agent should do.
- `scope`: global/domain/project/session.
- `confidence`: derived from recurrence, recency, and user confirmation.
- `status`: `tentative|pending_review|active|archived|rejected`.

Normalization flow:
1. Detect repeated pattern clusters from memories/corrections.
2. Build candidate rule with traceable evidence (`source_memory_ids`).
3. Auto-activate only above strict confidence threshold; otherwise keep `pending_review`.
4. Execute via read-path policy hooks (ranking boost, formatting constraints, routing).
5. Demote/archive rule when stale or contradicted.

This keeps rules auditable, reversible, and machine-executable.

## 8. Skill/MCP Recommendation Mining
Use memory aggregates to suggest business-useful capabilities:
- `skill` recommendation: reusable workflow appears repeatedly (e.g., frequent release-checklist pattern).
- `mcp` recommendation: repeated tool/API intent indicates integration gap (e.g., frequent manual Jira/GitHub/CRM steps).

Recommendation pipeline:
1. Mine high-frequency intent + pain-point clusters from active and archived memories.
2. Score impact via `frequency * time_saved * error_reduction`.
3. Emit `memory_recommendations` with concrete proposal, evidence, and expected ROI.
4. Require admin review before publishing to agent runtime.
5. Track acceptance/rejection to tune future recommendation quality.

## 9. Lease + Optimistic Lock Strategy
### Lease acquisition
1. Worker upserts/updates `scheduler_leases` for `job_name='memory-lifecycle'` where `lease_until < NOW()`.
2. Success means leadership for a short TTL (e.g. 30s).
3. Leader heartbeats every 10s (`lease_until = NOW()+30s`).

### Processing loop
1. Pull due `memory_jobs` in small batches.
2. For each candidate memory, read `(id, version, tier, state, salience_score, reuse_count, last_access_at)`.
3. Compute decision.
4. Apply with CAS update:
   `UPDATE memories SET tier=?, state=?, version=version+1 ... WHERE id=? AND version=?`.
5. If `RowsAffected=0`, treat as concurrent write; requeue with backoff.

This ensures no double-promotion/demotion even with concurrent API writes.

## 10. Knowledge Base Migration Capability
Support tenant-to-tenant migration with phases:
1. Snapshot: export source tenant memories in pages (`id`, content, metadata, tags, state, tier, timestamps).
2. Transform: normalize schema versions, optionally re-embed vectors if model/dims differ.
3. Load: bulk upsert into target tenant, preserving lineage (`origin_tenant`, `origin_id`).
4. Verify: count parity, checksum on canonical fields, random sample semantic checks.
5. Cutover (optional): dual-write window then switch read tenant.

Implementation options:
- API-level migration endpoint for managed flow.
- Offline CLI (`memorix migrate`) for large datasets and resumable transfer.

## 11. API & Config Additions
- New query filters: `tier`, `include_archived`, `min_salience`.
- New admin endpoints:
  - `POST /v1alpha1/tenants/{id}/lifecycle/run`
  - `GET /v1alpha1/tenants/{id}/lifecycle/runs`
  - `POST /v1alpha1/tenants/{id}/migrations`
  - `GET /v1alpha1/tenants/{id}/rules`
  - `POST /v1alpha1/tenants/{id}/rules/{ruleId}/activate`
  - `GET /v1alpha1/tenants/{id}/recommendations`
  - `POST /v1alpha1/tenants/{id}/recommendations/{recId}/review`
- Config:
  - `MNEMO_LIFECYCLE_ENABLED=true`
  - `MNEMO_LIFECYCLE_INTERVAL=5m`
  - `MNEMO_LEASE_TTL=30s`
  - `MNEMO_ARCHIVE_AFTER=30d`
  - `MNEMO_RULE_AUTO_ACTIVATE_MIN_CONF=0.85`
  - `MNEMO_RECOMMENDATION_ENABLED=true`

## 12. Rollout Plan
1. Schema migration (additive columns/tables only).
2. Read-path telemetry updates (`reuse_count`, `last_access_at`).
3. Introduce worker in observe-only mode (logs lifecycle and rule suggestions, no writes).
4. Enable promotion/demotion writes for a pilot tenant.
5. Enable archive-first forgetting.
6. Enable rule normalization in `pending_review` mode.
7. Enable recommendation mining and admin review workflow.
8. Release migration tooling and runbook.

## 13. Risks & Mitigations
- Hot-row contention on popular memories: batch updates and debounce access counters.
- Lease split-brain from clock skew: DB time (`NOW()`) only, short lease TTL, heartbeat guard.
- Re-embedding migration cost: allow deferred embedding regeneration and mixed-mode search during transition.
- Policy mistakes archiving useful memories: reversible archive, per-tenant thresholds, dry-run mode.
- Bad auto-rules causing behavior drift: keep low-confidence rules in review queue and require evidence-linked audit.
- Noisy recommendations: add minimum impact threshold, dedupe by intent cluster, and feedback loop from reviewer decisions.
