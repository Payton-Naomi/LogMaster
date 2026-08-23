ALTER TABLE logmaster_api.projects
    ADD COLUMN IF NOT EXISTS scheduling_priority INTEGER NOT NULL DEFAULT 0;

ALTER TABLE logmaster_api.projects DROP CONSTRAINT IF EXISTS projects_scheduling_priority_check;
ALTER TABLE logmaster_api.projects
    ADD CONSTRAINT projects_scheduling_priority_check CHECK (scheduling_priority BETWEEN -1000 AND 1000);

ALTER TABLE logmaster_api.parse_tasks DROP CONSTRAINT IF EXISTS parse_tasks_priority_check;
ALTER TABLE logmaster_api.parse_tasks
    ADD CONSTRAINT parse_tasks_priority_check CHECK (priority BETWEEN -100 AND 100);

CREATE INDEX IF NOT EXISTS idx_projects_scheduling_priority
    ON logmaster_api.projects (scheduling_priority DESC, id);
