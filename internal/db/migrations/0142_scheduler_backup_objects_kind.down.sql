-- Reverse 0142: restore the 0114 scheduler_tasks.kind CHECK.
--
-- The seeded backup_objects task violates the narrowed list, so its rows go first or the down
-- aborts mid-chain (same shape as 0091, 0104 and 0114). The backup's files under
-- AURA_BACKUP_DIR/objects are not touched.
DELETE FROM aura.agent_job_runs
WHERE task_id IN (
    SELECT id FROM aura.scheduler_tasks WHERE kind = 'backup_objects'
);

DELETE FROM aura.scheduler_tasks WHERE kind = 'backup_objects';

ALTER TABLE aura.scheduler_tasks DROP CONSTRAINT scheduler_tasks_kind_check;
ALTER TABLE aura.scheduler_tasks ADD CONSTRAINT scheduler_tasks_kind_check
    CHECK (kind IN ('reminder', 'agent_job', 'backup_postgres',
        'skill_ttl_sweep', 'identity_purge', 'sandbox_reap', 'share_expiry_sweep',
        'retention_sweep', 'memory_embed_backfill', 'observability_check',
        'memory_mention_link'));
