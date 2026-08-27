package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"mural-conservation-gate/internal/domain"
)

type CaseSearch struct {
	Status          domain.CaseStatus
	Keyword         string
	CursorUpdatedAt *time.Time
	CursorCaseID    string
	Limit           int
}

type CaseSearchResult struct {
	Cases        []domain.ConservationCase
	Total        int
	StatusCounts map[domain.CaseStatus]int
}

func (s *Store) SearchCases(ctx context.Context, query CaseSearch) (CaseSearchResult, error) {
	keyword := "%" + strings.ToLower(query.Keyword) + "%"
	cursor := ""
	if query.CursorUpdatedAt != nil {
		cursor = stamp(*query.CursorUpdatedAt)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT case_id,site_name,mural_location,material_layers,pathologies,ambient,baseline_revision_id,status,version,created_at,updated_at FROM cases WHERE (?='' OR status=?) AND (?='%%' OR lower(site_name) LIKE ? OR lower(mural_location) LIKE ?) AND (?='' OR updated_at<? OR (updated_at=? AND case_id>?)) ORDER BY updated_at DESC,case_id ASC LIMIT ?`, string(query.Status), string(query.Status), keyword, keyword, keyword, cursor, cursor, cursor, query.CursorCaseID, query.Limit)
	if err != nil {
		return CaseSearchResult{}, err
	}
	result := CaseSearchResult{Cases: []domain.ConservationCase{}, StatusCounts: map[domain.CaseStatus]int{}}
	for rows.Next() {
		var c domain.ConservationCase
		var layers, pathologies, ambient []byte
		var status, created, updated string
		if err = rows.Scan(&c.CaseID, &c.SiteName, &c.MuralLocation, &layers, &pathologies, &ambient, &c.BaselineRevisionID, &status, &c.Version, &created, &updated); err != nil {
			rows.Close()
			return result, err
		}
		c.Status = domain.CaseStatus(status)
		if err = decode(layers, &c.MaterialLayers); err != nil {
			rows.Close()
			return result, err
		}
		if err = decode(pathologies, &c.Pathologies); err != nil {
			rows.Close()
			return result, err
		}
		if err = decode(ambient, &c.AmbientCondition); err != nil {
			rows.Close()
			return result, err
		}
		if c.CreatedAt, err = parseStamp(created); err != nil {
			rows.Close()
			return result, err
		}
		if c.UpdatedAt, err = parseStamp(updated); err != nil {
			rows.Close()
			return result, err
		}
		result.Cases = append(result.Cases, c)
	}
	if err = rows.Close(); err != nil {
		return result, err
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cases WHERE (?='' OR status=?) AND (?='%%' OR lower(site_name) LIKE ? OR lower(mural_location) LIKE ?)`, string(query.Status), string(query.Status), keyword, keyword, keyword).Scan(&result.Total); err != nil {
		return result, err
	}
	countRows, err := s.db.QueryContext(ctx, `SELECT status,COUNT(*) FROM cases WHERE (?='%%' OR lower(site_name) LIKE ? OR lower(mural_location) LIKE ?) GROUP BY status`, keyword, keyword, keyword)
	if err != nil {
		return result, err
	}
	defer countRows.Close()
	for countRows.Next() {
		var status string
		var count int
		if err = countRows.Scan(&status, &count); err != nil {
			return result, err
		}
		result.StatusCounts[domain.CaseStatus(status)] = count
	}
	return result, countRows.Err()
}

func (s *Store) GetObservation(ctx context.Context, id string) (domain.TrialObservation, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM observations WHERE observation_id=?`, id).Scan(&data)
	if err == sql.ErrNoRows {
		return domain.TrialObservation{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.TrialObservation{}, err
	}
	var value domain.TrialObservation
	err = decode(data, &value)
	return value, err
}

func (s *Store) GetSnapshotByObservation(ctx context.Context, id string) (domain.EvaluationSnapshot, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM evaluation_snapshots WHERE observation_id=? ORDER BY evaluated_at DESC LIMIT 1`, id).Scan(&data)
	if err == sql.ErrNoRows {
		return domain.EvaluationSnapshot{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.EvaluationSnapshot{}, err
	}
	var value domain.EvaluationSnapshot
	err = decode(data, &value)
	return value, err
}

func (s *Store) RemediationQueue(ctx context.Context, assignee string, overdue bool, severity domain.Severity, now time.Time) ([]domain.RemediationTask, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM remediation_tasks WHERE (?='' OR assignee=?) AND (?='' OR severity=?) AND (?=0 OR (status<>? AND due_at<?)) ORDER BY due_at,task_id`, assignee, assignee, string(severity), string(severity), boolInt(overdue), domain.RemediationResolved, stamp(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.RemediationTask{}
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		var value domain.RemediationTask
		if err = decode(data, &value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
