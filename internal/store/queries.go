package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"mural-conservation-gate/internal/domain"
)

func (s *Store) GetCase(ctx context.Context, caseID string) (domain.ConservationCase, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.ConservationCase{}, err
	}
	defer tx.Rollback()
	return tx.LoadCase(ctx, caseID)
}

func (s *Store) GetView(ctx context.Context, caseID string) (domain.CaseView, error) {
	c, err := s.GetCase(ctx, caseID)
	if err != nil {
		return domain.CaseView{}, err
	}
	view := domain.CaseView{Case: c, Baselines: []domain.BaselineRevision{}, Zones: []domain.TrialZone{}, Protocols: []domain.CleaningProtocolRevision{}, Observations: []domain.TrialObservation{}, Findings: []domain.RiskFinding{}, Snapshots: []domain.EvaluationSnapshot{}, Remediations: []domain.RemediationTask{}}
	if err = queryPayloads(ctx, s.db, `SELECT payload FROM baselines WHERE case_id=? ORDER BY revision_no`, caseID, func(data []byte) error {
		var v domain.BaselineRevision
		if e := decode(data, &v); e != nil {
			return e
		}
		view.Baselines = append(view.Baselines, v)
		return nil
	}); err != nil {
		return view, err
	}
	if c.BaselineRevisionID != "" {
		var payload []byte
		if err = s.db.QueryRowContext(ctx, `SELECT payload FROM baselines WHERE revision_id=?`, c.BaselineRevisionID).Scan(&payload); err != nil {
			return view, err
		}
		var baseline domain.BaselineRevision
		if err = decode(payload, &baseline); err != nil {
			return view, err
		}
		view.Baseline = &baseline
	}
	if err = queryPayloads(ctx, s.db, `SELECT payload FROM zones WHERE case_id=? ORDER BY created_at`, caseID, func(data []byte) error {
		var v domain.TrialZone
		if e := decode(data, &v); e != nil {
			return e
		}
		view.Zones = append(view.Zones, v)
		return nil
	}); err != nil {
		return view, err
	}
	if err = queryPayloads(ctx, s.db, `SELECT payload FROM protocol_revisions WHERE case_id=? ORDER BY protocol_id,revision_no`, caseID, func(data []byte) error {
		var v domain.CleaningProtocolRevision
		if e := decode(data, &v); e != nil {
			return e
		}
		view.Protocols = append(view.Protocols, v)
		return nil
	}); err != nil {
		return view, err
	}
	if err = queryPayloads(ctx, s.db, `SELECT payload FROM observations WHERE case_id=? ORDER BY submitted_at`, caseID, func(data []byte) error {
		var v domain.TrialObservation
		if e := decode(data, &v); e != nil {
			return e
		}
		view.Observations = append(view.Observations, v)
		return nil
	}); err != nil {
		return view, err
	}
	if err = queryPayloads(ctx, s.db, `SELECT payload FROM findings WHERE case_id=? ORDER BY opened_at,rule_code`, caseID, func(data []byte) error {
		var v domain.RiskFinding
		if e := decode(data, &v); e != nil {
			return e
		}
		v.Historical = v.BaselineRevisionID != "" && v.BaselineRevisionID != c.BaselineRevisionID
		view.Findings = append(view.Findings, v)
		return nil
	}); err != nil {
		return view, err
	}
	if err = queryPayloads(ctx, s.db, `SELECT payload FROM evaluation_snapshots WHERE case_id=? ORDER BY evaluated_at`, caseID, func(data []byte) error {
		var v domain.EvaluationSnapshot
		if e := decode(data, &v); e != nil {
			return e
		}
		v.Historical = v.BaselineRevisionID != "" && v.BaselineRevisionID != c.BaselineRevisionID
		view.Snapshots = append(view.Snapshots, v)
		return nil
	}); err != nil {
		return view, err
	}
	if err = queryPayloads(ctx, s.db, `SELECT payload FROM remediation_tasks WHERE case_id=? ORDER BY due_at,task_id`, caseID, func(data []byte) error {
		var v domain.RemediationTask
		if e := decode(data, &v); e != nil {
			return e
		}
		view.Remediations = append(view.Remediations, v)
		return nil
	}); err != nil {
		return view, err
	}
	var selectionData []byte
	selectionErr := s.db.QueryRowContext(ctx, `SELECT payload FROM candidate_selections WHERE case_id=? ORDER BY selected_at DESC LIMIT 1`, caseID).Scan(&selectionData)
	if selectionErr == nil {
		var selection domain.CandidateSelection
		if err = decode(selectionData, &selection); err != nil {
			return view, err
		}
		view.Selection = &selection
	} else if !errors.Is(selectionErr, sql.ErrNoRows) {
		return view, selectionErr
	}
	view.Manifest, err = s.GetManifest(ctx, caseID)
	if errors.Is(err, domain.ErrNotFound) {
		err = nil
	}
	if err != nil {
		return view, err
	}
	view.Permit, err = s.GetPermitByCase(ctx, caseID)
	if errors.Is(err, domain.ErrNotFound) {
		err = nil
	}
	return view, err
}

type rowQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func queryPayloads(ctx context.Context, q rowQuery, statement string, argument any, consume func([]byte) error) error {
	rows, err := q.QueryContext(ctx, statement, argument)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var payload []byte
		if err = rows.Scan(&payload); err != nil {
			return err
		}
		if err = consume(payload); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Store) GetManifest(ctx context.Context, caseID string) (*domain.FrozenManifest, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM frozen_manifests WHERE case_id=?`, caseID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var value domain.FrozenManifest
	if err = decode(payload, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

func (s *Store) GetPermitByCase(ctx context.Context, caseID string) (*domain.WorkPermit, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM permits WHERE case_id=?`, caseID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var value domain.WorkPermit
	if err = decode(payload, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

func (s *Store) GetPermitByNumber(ctx context.Context, number string) (*domain.WorkPermit, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM permits WHERE permit_number=?`, number).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var value domain.WorkPermit
	if err = decode(payload, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

func (s *Store) Audit(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT sequence,event_type,actor_id,summary,payload,created_at FROM audit_events WHERE case_id=? ORDER BY sequence`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.AuditEvent{}
	for rows.Next() {
		var item domain.AuditEvent
		var created string
		if err = rows.Scan(&item.Sequence, &item.EventType, &item.ActorID, &item.Summary, &item.Payload, &created); err != nil {
			return nil, err
		}
		item.CaseID = caseID
		item.CreatedAt, err = parseStamp(created)
		if err != nil {
			return nil, err
		}
		values = append(values, item)
	}
	return values, rows.Err()
}

func (s *Store) LatestProtocol(ctx context.Context, caseID, protocolID string) (domain.CleaningProtocolRevision, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM protocol_revisions WHERE case_id=? AND protocol_id=? ORDER BY revision_no DESC LIMIT 1`, caseID, protocolID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CleaningProtocolRevision{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.CleaningProtocolRevision{}, err
	}
	var value domain.CleaningProtocolRevision
	if err = decode(payload, &value); err != nil {
		return value, err
	}
	return value, nil
}

func (s *Store) GetProtocolRevision(ctx context.Context, revisionID string) (domain.CleaningProtocolRevision, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM protocol_revisions WHERE revision_id=?`, revisionID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CleaningProtocolRevision{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.CleaningProtocolRevision{}, err
	}
	var value domain.CleaningProtocolRevision
	if err = decode(payload, &value); err != nil {
		return value, err
	}
	return value, nil
}

func (s *Store) CountOpenFindings(ctx context.Context, caseID, revisionID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM findings WHERE case_id=? AND protocol_revision_id=? AND status=?`, caseID, revisionID, domain.FindingOpen).Scan(&count)
	return count, err
}

func (s *Store) DebugSchemaVersion(ctx context.Context) (int, error) {
	var version int
	err := s.db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("读取迁移版本: %w", err)
	}
	return version, nil
}
