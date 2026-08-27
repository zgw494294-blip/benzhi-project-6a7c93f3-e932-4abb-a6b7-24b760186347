package workflow

import (
	"context"
	"fmt"

	"mural-conservation-gate/internal/domain"
	"mural-conservation-gate/internal/evaluation"
	"mural-conservation-gate/internal/store"
)

func (s *Service) SubmitObservation(ctx context.Context, caseID string, cmd ObservationCommand) (domain.CaseView, error) {
	return s.mutate(ctx, caseID, cmd.ExpectedVersion, func(tx *store.Tx, view domain.CaseView) (domain.CaseStatus, string, error) {
		if err := ensureState(view.Case, domain.StatusTesting, domain.StatusRemediation, domain.StatusReview); err != nil {
			return "", "", err
		}
		if view.Baseline == nil {
			return "", "", fmt.Errorf("%w: 未提交病害基线", domain.ErrIncomplete)
		}
		zone, ok := findZone(view, cmd.ZoneID)
		if !ok {
			return "", "", domain.Invalid("zoneId", "分区不存在")
		}
		protocol, ok := findProtocol(view, cmd.ProtocolRevisionID)
		if !ok {
			return "", "", domain.Invalid("protocolRevisionId", "方案修订不存在")
		}
		observation := domain.TrialObservation{ObservationID: newID("observation"), CaseID: caseID, ZoneID: cmd.ZoneID, ProtocolRevisionID: cmd.ProtocolRevisionID, BaselineRevisionID: view.Case.BaselineRevisionID, RoundNo: cmd.RoundNo, ObservedAt: cmd.ObservedAt, ColorDeltaE: cmd.ColorDeltaE, ParticleLossScore: cmd.ParticleLossScore, MoisturePercent: cmd.MoisturePercent, ResidueValue: cmd.ResidueValue, EvidenceDigest: cmd.EvidenceDigest, EvidenceSummary: cmd.EvidenceSummary, OperatorID: cmd.OperatorID, SubmittedAt: s.now()}
		if err := domain.ValidateObservation(observation); err != nil {
			return "", "", err
		}
		if err := tx.InsertObservation(ctx, observation); err != nil {
			return "", "", normalizeStoreError(err, "roundNo")
		}
		status := domain.StatusTesting
		if zone.ZoneType == domain.ZoneTrial {
			controls := controlObservations(view, zone.ControlZoneID, protocol.RevisionID, view.Case.BaselineRevisionID)
			open := currentOpenFindings(view.Findings)
			result := s.eval.Evaluate(evaluation.Input{Case: view.Case, Baseline: *view.Baseline, Protocol: protocol, Current: observation, History: view.Observations, Controls: controls, OpenFindings: open})
			for i := range result.Findings {
				finding := result.Findings[i]
				if finding.Status == domain.FindingClosed {
					for _, old := range open {
						if old.FindingID == finding.FindingID && old.ProtocolRevisionID == protocol.RevisionID {
							finding = old
						}
					}
				}
				if err := tx.UpsertFinding(ctx, finding); err != nil {
					return "", "", err
				}
			}
			if err := tx.InsertSnapshot(ctx, result.Snapshot); err != nil {
				return "", "", err
			}
			if result.HasBlockers || hasOpenAfter(result.Findings) {
				status = domain.StatusRemediation
			} else {
				status = domain.StatusReview
			}
		}
		if err := tx.AppendAudit(ctx, caseID, "observation.submitted", observation.OperatorID, "登记不可变试验观察并执行风险评估", observation); err != nil {
			return "", "", err
		}
		return status, "", nil
	})
}

func controlObservations(view domain.CaseView, controlZoneID, protocolRevisionID, baselineRevisionID string) []domain.TrialObservation {
	values := []domain.TrialObservation{}
	for _, item := range view.Observations {
		if item.ZoneID == controlZoneID && item.ProtocolRevisionID == protocolRevisionID && item.BaselineRevisionID == baselineRevisionID {
			values = append(values, item)
		}
	}
	return values
}

func currentOpenFindings(values []domain.RiskFinding) []domain.RiskFinding {
	result := []domain.RiskFinding{}
	for _, item := range values {
		if item.Status == domain.FindingOpen && !item.Historical {
			result = append(result, item)
		}
	}
	return result
}

func hasOpenAfter(values []domain.RiskFinding) bool {
	for _, item := range values {
		if item.Status == domain.FindingOpen && item.Severity == domain.SeverityBlocking {
			return true
		}
	}
	return false
}
