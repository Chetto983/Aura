-- Admit the nightly object-store backup as a scheduler kind.
--
-- backup_objects copies every bucket Aura holds a key for into AURA_BACKUP_DIR/objects
-- (docs/superpowers/specs/2026-10-19-object-backup-design.md). seedObjectBackup seeds it and
-- handlers.NewObjectBackupHandler runs it. The 2026-10-14 reinstall of the lab VM restored
-- Postgres and ArcadeDB and nothing restored Garage's objects.
--
-- The 0009 constraint is unnamed (inline column CHECK), so Postgres auto-named it
-- `scheduler_tasks_kind_check`; drop + re-add it with the extra member, exactly as 0091, 0104
-- and 0114 did. The member list is 0114's (the latest migration to touch this constraint) plus
-- 'backup_objects'. 0009 GRANTed aura_app DML on scheduler_tasks already.

ALTER TABLE aura.scheduler_tasks DROP CONSTRAINT scheduler_tasks_kind_check;
ALTER TABLE aura.scheduler_tasks ADD CONSTRAINT scheduler_tasks_kind_check
    CHECK (kind IN ('reminder', 'agent_job', 'backup_postgres',
        'skill_ttl_sweep', 'identity_purge', 'sandbox_reap', 'share_expiry_sweep',
        'retention_sweep', 'memory_embed_backfill', 'observability_check',
        'memory_mention_link', 'backup_objects'));
