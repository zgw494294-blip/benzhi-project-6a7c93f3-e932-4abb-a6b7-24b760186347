package workflow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"mural-conservation-gate/internal/domain"
	"mural-conservation-gate/internal/evaluation"
	"mural-conservation-gate/internal/store"
)

type Service struct {
	store             *store.Store
	eval              *evaluation.Evaluator
	locks             *caseLocks
	now               func() time.Time
	comparisonMu      sync.RWMutex
	comparisonResults map[comparisonCacheKey]domain.CandidateComparisonSet
}

type comparisonCacheKey struct {
	caseID  string
	version int64
}

func New(s *store.Store) *Service {
	now := func() time.Time { return time.Now().UTC() }
	service := &Service{store: s, locks: newCaseLocks(), now: now, comparisonResults: map[comparisonCacheKey]domain.CandidateComparisonSet{}}
	service.eval = evaluation.New(now, newID)
	return service
}

func newID(prefix string) string {
	data := make([]byte, 12)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(data)
}

func (s *Service) Store() *store.Store { return s.store }

func (s *Service) CreateCase(ctx context.Context, cmd CreateCaseCommand) (domain.ConservationCase, error) {
	now := s.now()
	c := domain.ConservationCase{CaseID: newID("case"), SiteName: strings.TrimSpace(cmd.SiteName), MuralLocation: strings.TrimSpace(cmd.MuralLocation), MaterialLayers: cmd.MaterialLayers, Pathologies: cmd.Pathologies, AmbientCondition: cmd.AmbientCondition, Status: domain.StatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := domain.ValidateNewCase(c); err != nil {
		return c, err
	}
	if strings.TrimSpace(cmd.ActorID) == "" {
		return c, domain.Invalid("actorId", "必须登记建档人员")
	}
	tx, err := s.store.Begin(ctx)
	if err != nil {
		return c, err
	}
	defer tx.Rollback()
	if err = tx.InsertCase(ctx, c, cmd.ActorID); err != nil {
		return c, err
	}
	if err = tx.Commit(); err != nil {
		return c, err
	}
	return c, nil
}

func (s *Service) GetCase(ctx context.Context, caseID string) (domain.CaseView, error) {
	view, err := s.store.GetView(ctx, caseID)
	if err == nil {
		comparison := evaluation.CompareCandidates(view)
		if view.Selection != nil {
			view.Selection.Valid = false
			view.Selection.InvalidReason = "候选方案、证据或风险状态已变化，需要重新比选"
			for _, candidate := range comparison.Candidates {
				if candidate.ProtocolRevisionID == view.Selection.ProtocolRevisionID && candidate.Eligible && comparison.Digest == view.Selection.ComparisonDigest {
					view.Selection.Valid = true
					view.Selection.InvalidReason = ""
				}
			}
		}
		view.Readiness = domain.AssessReadiness(view)
	}
	return view, err
}

func (s *Service) CandidateComparisons(ctx context.Context, caseID string) (domain.CandidateComparisonSet, error) {
	view, err := s.GetCase(ctx, caseID)
	if err != nil {
		return domain.CandidateComparisonSet{}, err
	}
	key := comparisonCacheKey{caseID: caseID, version: view.Case.Version}
	s.comparisonMu.RLock()
	result, ok := s.comparisonResults[key]
	s.comparisonMu.RUnlock()
	if ok {
		return result, nil
	}
	result = evaluation.CompareCandidates(view)
	s.comparisonMu.Lock()
	s.comparisonResults[key] = result
	s.comparisonMu.Unlock()
	return result, nil
}

func (s *Service) Audit(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	return s.store.Audit(ctx, caseID)
}

func requireExpected(actual, expected int64) error {
	if expected < 1 {
		return domain.Invalid("expectedVersion", "必须提供正整数版本号")
	}
	if actual != expected {
		return fmt.Errorf("%w: 当前版本为 %d", domain.ErrConflict, actual)
	}
	return nil
}

func findZone(view domain.CaseView, id string) (domain.TrialZone, bool) {
	for _, item := range view.Zones {
		if item.ZoneID == id {
			return item, true
		}
	}
	return domain.TrialZone{}, false
}

func findProtocol(view domain.CaseView, id string) (domain.CleaningProtocolRevision, bool) {
	for _, item := range view.Protocols {
		if item.RevisionID == id {
			return item, true
		}
	}
	return domain.CleaningProtocolRevision{}, false
}

func (s *Service) mutate(ctx context.Context, caseID string, expected int64, action func(*store.Tx, domain.CaseView) (domain.CaseStatus, string, error)) (domain.CaseView, error) {
	unlock := s.locks.lock(caseID)
	defer unlock()
	view, err := s.store.GetView(ctx, caseID)
	if err != nil {
		return view, err
	}
	if err = requireExpected(view.Case.Version, expected); err != nil {
		return view, err
	}
	if err = domain.CanModify(view.Case.Status); err != nil {
		return view, err
	}
	tx, err := s.store.Begin(ctx)
	if err != nil {
		return view, err
	}
	defer tx.Rollback()
	status, baselineID, err := action(tx, view)
	if err != nil {
		return view, err
	}
	if status == "" {
		status = view.Case.Status
	}
	if err = tx.AdvanceCase(ctx, &view.Case, expected, status, baselineID); err != nil {
		return view, err
	}
	if err = tx.Commit(); err != nil {
		return view, err
	}
	return s.GetCase(ctx, caseID)
}

func ensureState(c domain.ConservationCase, allowed ...domain.CaseStatus) error {
	for _, status := range allowed {
		if c.Status == status {
			return nil
		}
	}
	return fmt.Errorf("%w: 当前状态为 %s", domain.ErrInvalidState, c.Status)
}

func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func normalizeStoreError(err error, field string) error {
	if isUniqueConstraint(err) {
		return domain.Invalid(field, "相同标识或轮次已经存在，不可原地改写")
	}
	return err
}
