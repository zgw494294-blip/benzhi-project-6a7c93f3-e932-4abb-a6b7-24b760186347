package store

const schema = `
PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(1, CURRENT_TIMESTAMP);
INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(2, CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS cases(
 case_id TEXT PRIMARY KEY, site_name TEXT NOT NULL, mural_location TEXT NOT NULL,
 material_layers BLOB NOT NULL, pathologies BLOB NOT NULL, ambient BLOB NOT NULL,
 baseline_revision_id TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, version INTEGER NOT NULL,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS baselines(
 revision_id TEXT PRIMARY KEY, case_id TEXT NOT NULL, revision_no INTEGER NOT NULL,
 payload BLOB NOT NULL, created_at TEXT NOT NULL, UNIQUE(case_id, revision_no),
 FOREIGN KEY(case_id) REFERENCES cases(case_id)
);
CREATE TABLE IF NOT EXISTS zones(
 zone_id TEXT PRIMARY KEY, case_id TEXT NOT NULL, zone_type TEXT NOT NULL,
 control_zone_id TEXT NOT NULL DEFAULT '', payload BLOB NOT NULL, created_at TEXT NOT NULL,
 FOREIGN KEY(case_id) REFERENCES cases(case_id)
);
CREATE TABLE IF NOT EXISTS protocol_revisions(
 revision_id TEXT PRIMARY KEY, case_id TEXT NOT NULL, protocol_id TEXT NOT NULL,
 revision_no INTEGER NOT NULL, payload BLOB NOT NULL, created_at TEXT NOT NULL,
 UNIQUE(case_id, protocol_id, revision_no), FOREIGN KEY(case_id) REFERENCES cases(case_id)
);
CREATE TABLE IF NOT EXISTS observations(
 observation_id TEXT PRIMARY KEY, case_id TEXT NOT NULL, zone_id TEXT NOT NULL,
 protocol_revision_id TEXT NOT NULL, round_no INTEGER NOT NULL, payload BLOB NOT NULL,
 submitted_at TEXT NOT NULL, UNIQUE(case_id, zone_id, protocol_revision_id, round_no),
 FOREIGN KEY(case_id) REFERENCES cases(case_id)
);
CREATE TABLE IF NOT EXISTS evaluation_snapshots(
 snapshot_id TEXT PRIMARY KEY, case_id TEXT NOT NULL, observation_id TEXT NOT NULL,
 payload BLOB NOT NULL, evaluated_at TEXT NOT NULL, FOREIGN KEY(case_id) REFERENCES cases(case_id)
);
CREATE TABLE IF NOT EXISTS findings(
 finding_id TEXT PRIMARY KEY, case_id TEXT NOT NULL, protocol_revision_id TEXT NOT NULL,
 rule_code TEXT NOT NULL, status TEXT NOT NULL, payload BLOB NOT NULL,
 opened_at TEXT NOT NULL, UNIQUE(case_id, protocol_revision_id, rule_code),
 FOREIGN KEY(case_id) REFERENCES cases(case_id)
);
CREATE TABLE IF NOT EXISTS frozen_manifests(
 manifest_id TEXT PRIMARY KEY, case_id TEXT NOT NULL UNIQUE, protocol_revision_id TEXT NOT NULL,
 digest TEXT NOT NULL, payload BLOB NOT NULL, frozen_at TEXT NOT NULL,
 FOREIGN KEY(case_id) REFERENCES cases(case_id)
);
CREATE TABLE IF NOT EXISTS permits(
 permit_id TEXT PRIMARY KEY, permit_number TEXT NOT NULL UNIQUE, case_id TEXT NOT NULL UNIQUE,
 verification_digest TEXT NOT NULL, payload BLOB NOT NULL, issued_at TEXT NOT NULL,
 FOREIGN KEY(case_id) REFERENCES cases(case_id)
);
CREATE TABLE IF NOT EXISTS idempotency_records(
 idempotency_key TEXT PRIMARY KEY, request_fingerprint TEXT NOT NULL,
 response BLOB NOT NULL, status_code INTEGER NOT NULL, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS audit_events(
 sequence INTEGER PRIMARY KEY AUTOINCREMENT, case_id TEXT NOT NULL, event_type TEXT NOT NULL,
 actor_id TEXT NOT NULL, summary TEXT NOT NULL, payload BLOB NOT NULL, created_at TEXT NOT NULL,
 FOREIGN KEY(case_id) REFERENCES cases(case_id)
);
CREATE TABLE IF NOT EXISTS remediation_tasks(
 task_id TEXT PRIMARY KEY, case_id TEXT NOT NULL, finding_id TEXT NOT NULL UNIQUE,
 assignee TEXT NOT NULL, severity TEXT NOT NULL, status TEXT NOT NULL, due_at TEXT NOT NULL,
 payload BLOB NOT NULL, updated_at TEXT NOT NULL, FOREIGN KEY(case_id) REFERENCES cases(case_id)
);
CREATE TABLE IF NOT EXISTS candidate_selections(
 selection_id TEXT PRIMARY KEY, case_id TEXT NOT NULL, protocol_revision_id TEXT NOT NULL,
 comparison_digest TEXT NOT NULL, payload BLOB NOT NULL, selected_at TEXT NOT NULL,
 FOREIGN KEY(case_id) REFERENCES cases(case_id)
);
CREATE INDEX IF NOT EXISTS idx_zones_case ON zones(case_id);
CREATE INDEX IF NOT EXISTS idx_protocols_case ON protocol_revisions(case_id, protocol_id, revision_no);
CREATE INDEX IF NOT EXISTS idx_observations_case ON observations(case_id, submitted_at);
CREATE INDEX IF NOT EXISTS idx_findings_case ON findings(case_id, status);
CREATE INDEX IF NOT EXISTS idx_audit_case ON audit_events(case_id, sequence);
CREATE INDEX IF NOT EXISTS idx_cases_queue ON cases(updated_at DESC, case_id ASC);
CREATE INDEX IF NOT EXISTS idx_remediation_queue ON remediation_tasks(status, due_at, assignee, severity);
CREATE INDEX IF NOT EXISTS idx_selections_case ON candidate_selections(case_id, selected_at DESC);
`
