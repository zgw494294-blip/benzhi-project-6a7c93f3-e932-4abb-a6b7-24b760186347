package workflow

import (
	"context"
	"fmt"
	"strings"

	"mural-conservation-gate/internal/domain"
	"mural-conservation-gate/internal/store"
)

func (s *Service) SubmitBaseline(ctx context.Context, caseID string, cmd BaselineCommand) (domain.CaseView, error) {
	return s.mutate(ctx, caseID, cmd.ExpectedVersion, func(tx *store.Tx, view domain.CaseView) (domain.CaseStatus, string, error) {
		if err := ensureState(view.Case, domain.StatusDraft, domain.StatusTesting, domain.StatusRemediation, domain.StatusReview); err != nil {
			return "", "", err
		}
		revisionNo := 1
		if view.Baseline != nil {
			revisionNo = view.Baseline.RevisionNo + 1
		}
		revision := domain.BaselineRevision{RevisionID: newID("baseline"), RevisionNo: revisionNo, ColorL: cmd.ColorL, ColorA: cmd.ColorA, ColorB: cmd.ColorB, MoisturePercent: cmd.MoisturePercent, ParticleCondition: strings.TrimSpace(cmd.ParticleCondition), ResidueBaseline: cmd.ResidueBaseline, MeasuredAt: cmd.MeasuredAt, MeasuredBy: strings.TrimSpace(cmd.MeasuredBy), Ambient: cmd.Ambient, CreatedAt: s.now(), Reason: strings.TrimSpace(cmd.Reason)}
		if err := domain.ValidateBaseline(revision); err != nil {
			return "", "", err
		}
		eventType, summary, status := "baseline.submitted", "提交完整病害基线版本", domain.StatusTesting
		if view.Baseline != nil {
			if err := domain.ValidateBaselineRetest(*view.Baseline, revision); err != nil {
				return "", "", err
			}
			impact := domain.CalculateBaselineImpact(*view.Baseline, revision)
			revision.Impact = &impact
			eventType, summary = "baseline.retested", "追加不可变基线复测并使旧基线评估转为历史结论"
		}
		if err := tx.InsertBaselineForCase(ctx, caseID, revision); err != nil {
			return "", "", normalizeStoreError(err, "revisionNo")
		}
		if err := tx.AppendAudit(ctx, caseID, eventType, revision.MeasuredBy, summary, revision); err != nil {
			return "", "", err
		}
		return status, revision.RevisionID, nil
	})
}

func (s *Service) AddZone(ctx context.Context, caseID string, cmd ZoneCommand) (domain.CaseView, error) {
	return s.mutate(ctx, caseID, cmd.ExpectedVersion, func(tx *store.Tx, view domain.CaseView) (domain.CaseStatus, string, error) {
		if err := ensureState(view.Case, domain.StatusTesting, domain.StatusRemediation, domain.StatusReview); err != nil {
			return "", "", err
		}
		if view.Baseline == nil {
			return "", "", fmt.Errorf("%w: 未提交病害基线", domain.ErrIncomplete)
		}
		zone := domain.TrialZone{ZoneID: newID("zone"), CaseID: caseID, ZoneType: cmd.ZoneType, BoundaryPoints: cmd.BoundaryPoints, AreaCM2: cmd.AreaCM2, RepresentativeReason: strings.TrimSpace(cmd.RepresentativeReason), ControlZoneID: cmd.ControlZoneID, CreatedAt: s.now()}
		if err := domain.ValidateZone(zone); err != nil {
			return "", "", err
		}
		if zone.ZoneType == domain.ZoneTrial {
			control, ok := findZone(view, zone.ControlZoneID)
			if !ok || control.ZoneType != domain.ZoneControl {
				return "", "", domain.Invalid("controlZoneId", "关联的对照区不存在")
			}
		}
		for _, existing := range view.Zones {
			if domain.ZonesOverlap(existing, zone) {
				return "", "", domain.Invalid("boundaryPoints", "分区边界与已有分区重叠")
			}
		}
		if err := tx.InsertZone(ctx, zone); err != nil {
			return "", "", err
		}
		if err := tx.AppendAudit(ctx, caseID, "zone.created", cmd.ActorID, "划定清洗试验或对照分区", zone); err != nil {
			return "", "", err
		}
		return domain.StatusTesting, "", nil
	})
}

func (s *Service) ReviseProtocol(ctx context.Context, caseID string, cmd ProtocolCommand) (domain.CaseView, error) {
	return s.mutate(ctx, caseID, cmd.ExpectedVersion, func(tx *store.Tx, view domain.CaseView) (domain.CaseStatus, string, error) {
		if err := ensureState(view.Case, domain.StatusTesting, domain.StatusRemediation, domain.StatusReview); err != nil {
			return "", "", err
		}
		if view.Baseline == nil {
			return "", "", fmt.Errorf("%w: 未提交病害基线", domain.ErrIncomplete)
		}
		hasTrial, hasControl := false, false
		for _, zone := range view.Zones {
			hasTrial = hasTrial || zone.ZoneType == domain.ZoneTrial
			hasControl = hasControl || zone.ZoneType == domain.ZoneControl
		}
		if !hasTrial || !hasControl {
			return "", "", fmt.Errorf("%w: 必须先建立试验区和对照区", domain.ErrIncomplete)
		}
		revisionNo := 1
		for _, old := range view.Protocols {
			if old.ProtocolID == cmd.ProtocolID && old.RevisionNo >= revisionNo {
				revisionNo = old.RevisionNo + 1
			}
		}
		if revisionNo > 1 && strings.TrimSpace(cmd.ChangeReason) == "" {
			return "", "", domain.Invalid("changeReason", "修订候选方案必须说明原因")
		}
		value := domain.CleaningProtocolRevision{RevisionID: newID("protocolrev"), CaseID: caseID, ProtocolID: cmd.ProtocolID, RevisionNo: revisionNo, Ingredients: cmd.Ingredients, Concentration: cmd.Concentration, ContactSeconds: cmd.ContactSeconds, Tools: cmd.Tools, RemovalSteps: cmd.RemovalSteps, SafetyThresholds: cmd.SafetyThresholds, ChangeReason: strings.TrimSpace(cmd.ChangeReason), CreatedBy: strings.TrimSpace(cmd.CreatedBy), CreatedAt: s.now()}
		if err := domain.ValidateProtocol(value); err != nil {
			return "", "", err
		}
		if err := tx.InsertProtocol(ctx, value); err != nil {
			return "", "", normalizeStoreError(err, "protocolId")
		}
		for _, task := range view.Remediations {
			if task.Status != domain.RemediationPending {
				continue
			}
			source, ok := findProtocol(view, task.SourceProtocolRevisionID)
			if !ok || source.ProtocolID != value.ProtocolID {
				continue
			}
			expectedConcentration := source.Concentration
			if task.PlannedParameters.Concentration != nil {
				expectedConcentration = *task.PlannedParameters.Concentration
			}
			expectedContactSeconds := source.ContactSeconds
			if task.PlannedParameters.ContactSeconds != nil {
				expectedContactSeconds = *task.PlannedParameters.ContactSeconds
			}
			if value.Concentration != expectedConcentration {
				return "", "", domain.Invalid("concentration", "新方案参数与整改计划不一致")
			}
			if value.ContactSeconds != expectedContactSeconds {
				return "", "", domain.Invalid("contactSeconds", "新方案参数与整改计划不一致")
			}
			task.Status = domain.RemediationRetest
			task.RetestProtocolRevisionID = value.RevisionID
			task.UpdatedAt = s.now()
			findingSeverity := domain.SeverityBlocking
			for _, finding := range view.Findings {
				if finding.FindingID == task.FindingID {
					findingSeverity = finding.Severity
				}
			}
			if err := tx.UpsertRemediation(ctx, task, findingSeverity); err != nil {
				return "", "", err
			}
			if err := tx.AppendAudit(ctx, caseID, "remediation.awaiting_retest", value.CreatedBy, "方案参数已按整改计划修订，任务进入待复验", task); err != nil {
				return "", "", err
			}
		}
		if err := tx.AppendAudit(ctx, caseID, "protocol.revised", value.CreatedBy, "保存不可变候选清洗方案修订", value); err != nil {
			return "", "", err
		}
		return domain.StatusTesting, "", nil
	})
}
