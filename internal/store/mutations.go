package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"mural-conservation-gate/internal/domain"
)

func (t *Tx) InsertCase(ctx context.Context, c domain.ConservationCase, actor string) error {
	layers, _ := encode(c.MaterialLayers)
	pathologies, _ := encode(c.Pathologies)
	ambient, _ := encode(c.AmbientCondition)
	_, err := t.tx.ExecContext(ctx, `INSERT INTO cases(case_id,site_name,mural_location,material_layers,pathologies,ambient,baseline_revision_id,status,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		c.CaseID, c.SiteName, c.MuralLocation, layers, pathologies, ambient, c.BaselineRevisionID, c.Status, c.Version, stamp(c.CreatedAt), stamp(c.UpdatedAt))
	if err != nil {
		return fmt.Errorf("保存案卷: %w", err)
	}
	return t.AppendAudit(ctx, c.CaseID, "case.created", actor, "建立壁画保护作业案", c)
}

func (t *Tx) LoadCase(ctx context.Context, caseID string) (domain.ConservationCase, error) {
	var c domain.ConservationCase
	var layers, pathologies, ambient []byte
	var status, created, updated string
	err := t.tx.QueryRowContext(ctx, `SELECT case_id,site_name,mural_location,material_layers,pathologies,ambient,baseline_revision_id,status,version,created_at,updated_at FROM cases WHERE case_id=?`, caseID).
		Scan(&c.CaseID, &c.SiteName, &c.MuralLocation, &layers, &pathologies, &ambient, &c.BaselineRevisionID, &status, &c.Version, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return c, domain.ErrNotFound
	}
	if err != nil {
		return c, err
	}
	c.Status = domain.CaseStatus(status)
	if err = decode(layers, &c.MaterialLayers); err != nil {
		return c, err
	}
	if err = decode(pathologies, &c.Pathologies); err != nil {
		return c, err
	}
	if err = decode(ambient, &c.AmbientCondition); err != nil {
		return c, err
	}
	c.CreatedAt, err = parseStamp(created)
	if err != nil {
		return c, err
	}
	c.UpdatedAt, err = parseStamp(updated)
	return c, err
}

func (t *Tx) AdvanceCase(ctx context.Context, c *domain.ConservationCase, expected int64, status domain.CaseStatus, baselineID string) error {
	now := time.Now().UTC()
	result, err := t.tx.ExecContext(ctx, `UPDATE cases SET status=?, baseline_revision_id=CASE WHEN ?='' THEN baseline_revision_id ELSE ? END, version=version+1, updated_at=? WHERE case_id=? AND version=?`, status, baselineID, baselineID, stamp(now), c.CaseID, expected)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return domain.ErrConflict
	}
	c.Version = expected + 1
	c.Status = status
	c.UpdatedAt = now
	if baselineID != "" {
		c.BaselineRevisionID = baselineID
	}
	return nil
}

func (t *Tx) InsertBaselineForCase(ctx context.Context, caseID string, value domain.BaselineRevision) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO baselines(revision_id,case_id,revision_no,payload,created_at) VALUES(?,?,?,?,?)`, value.RevisionID, caseID, value.RevisionNo, payload, stamp(value.CreatedAt))
	return err
}

func (t *Tx) InsertZone(ctx context.Context, value domain.TrialZone) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO zones(zone_id,case_id,zone_type,control_zone_id,payload,created_at) VALUES(?,?,?,?,?,?)`, value.ZoneID, value.CaseID, value.ZoneType, value.ControlZoneID, payload, stamp(value.CreatedAt))
	return err
}

func (t *Tx) InsertProtocol(ctx context.Context, value domain.CleaningProtocolRevision) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO protocol_revisions(revision_id,case_id,protocol_id,revision_no,payload,created_at) VALUES(?,?,?,?,?,?)`, value.RevisionID, value.CaseID, value.ProtocolID, value.RevisionNo, payload, stamp(value.CreatedAt))
	return err
}

func (t *Tx) InsertObservation(ctx context.Context, value domain.TrialObservation) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO observations(observation_id,case_id,zone_id,protocol_revision_id,round_no,payload,submitted_at) VALUES(?,?,?,?,?,?,?)`, value.ObservationID, value.CaseID, value.ZoneID, value.ProtocolRevisionID, value.RoundNo, payload, stamp(value.SubmittedAt))
	return err
}

func (t *Tx) InsertSnapshot(ctx context.Context, value domain.EvaluationSnapshot) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO evaluation_snapshots(snapshot_id,case_id,observation_id,payload,evaluated_at) VALUES(?,?,?,?,?)`, value.SnapshotID, value.CaseID, value.ObservationID, payload, stamp(value.EvaluatedAt))
	return err
}

func (t *Tx) UpsertFinding(ctx context.Context, value domain.RiskFinding) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	storageRuleCode := value.RuleCode + "\x1f" + value.BaselineRevisionID
	_, err = t.tx.ExecContext(ctx, `INSERT INTO findings(finding_id,case_id,protocol_revision_id,rule_code,status,payload,opened_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(case_id,protocol_revision_id,rule_code) DO UPDATE SET finding_id=excluded.finding_id,status=excluded.status,payload=excluded.payload`, value.FindingID, value.CaseID, value.ProtocolRevisionID, storageRuleCode, value.Status, payload, stamp(value.OpenedAt))
	return err
}

func (t *Tx) InsertManifest(ctx context.Context, value domain.FrozenManifest) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO frozen_manifests(manifest_id,case_id,protocol_revision_id,digest,payload,frozen_at) VALUES(?,?,?,?,?,?)`, value.ManifestID, value.CaseID, value.ProtocolRevisionID, value.Digest, payload, stamp(value.FrozenAt))
	return err
}

func (t *Tx) InsertPermit(ctx context.Context, value domain.WorkPermit) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO permits(permit_id,permit_number,case_id,verification_digest,payload,issued_at) VALUES(?,?,?,?,?,?)`, value.PermitID, value.PermitNumber, value.CaseID, value.VerificationDigest, payload, stamp(value.IssuedAt))
	return err
}

func (s *Store) SavePermit(ctx context.Context, value domain.WorkPermit) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = tx.InsertPermit(ctx, value); err != nil {
		return err
	}
	return tx.Commit()
}

func (t *Tx) InsertSelection(ctx context.Context, value domain.CandidateSelection) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO candidate_selections(selection_id,case_id,protocol_revision_id,comparison_digest,payload,selected_at) VALUES(?,?,?,?,?,?)`, value.SelectionID, value.CaseID, value.ProtocolRevisionID, value.ComparisonDigest, payload, stamp(value.SelectedAt))
	return err
}

func (t *Tx) UpsertRemediation(ctx context.Context, value domain.RemediationTask, severity domain.Severity) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO remediation_tasks(task_id,case_id,finding_id,assignee,severity,status,due_at,payload,updated_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(finding_id) DO UPDATE SET assignee=excluded.assignee,severity=excluded.severity,status=excluded.status,due_at=excluded.due_at,payload=excluded.payload,updated_at=excluded.updated_at`, value.TaskID, value.CaseID, value.FindingID, value.Assignee, severity, value.Status, stamp(value.DueAt), payload, stamp(value.UpdatedAt))
	return err
}

func (t *Tx) AppendAudit(ctx context.Context, caseID, eventType, actor, summary string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO audit_events(case_id,event_type,actor_id,summary,payload,created_at) VALUES(?,?,?,?,?,?)`, caseID, eventType, actor, summary, data, stamp(time.Now().UTC()))
	return err
}
