package evaluation

import (
	"fmt"
	"sort"
	"time"

	"mural-conservation-gate/internal/domain"
)

type IDGenerator func(prefix string) string

type Evaluator struct {
	now func() time.Time
	id  IDGenerator
}

func New(now func() time.Time, id IDGenerator) *Evaluator {
	return &Evaluator{now: now, id: id}
}

type Input struct {
	Case         domain.ConservationCase
	Baseline     domain.BaselineRevision
	Protocol     domain.CleaningProtocolRevision
	Current      domain.TrialObservation
	History      []domain.TrialObservation
	Controls     []domain.TrialObservation
	OpenFindings []domain.RiskFinding
}

type Result struct {
	Snapshot    domain.EvaluationSnapshot
	Findings    []domain.RiskFinding
	HasBlockers bool
}

func (e *Evaluator) Evaluate(input Input) Result {
	now := e.now().UTC()
	violations := map[string]domain.RiskFinding{}
	for _, rule := range thresholdRules {
		value := rule.Value(input.Current)
		limit := rule.Threshold(input.Protocol.SafetyThresholds)
		if value > limit {
			violations[rule.Code] = e.finding(input, rule.Code, rule.Severity, fmt.Sprintf("%s：观测值 %.3f，阈值 %.3f", rule.Message, value, limit), now)
		}
	}
	if control, ok := matchingControl(input); ok {
		difference := input.Current.ColorDeltaE - control.ColorDeltaE
		if difference > input.Protocol.SafetyThresholds.MaxControlDifference {
			violations["CONTROL_COLOR_DIFFERENCE"] = e.finding(input, "CONTROL_COLOR_DIFFERENCE", domain.SeverityBlocking, fmt.Sprintf("试验区与对照区色差增量偏差 %.3f 超过阈值", difference), now)
		}
		residueDifference := input.Current.ResidueValue - control.ResidueValue
		if residueDifference > input.Protocol.SafetyThresholds.MaxControlDifference {
			violations["CONTROL_RESIDUE_DIFFERENCE"] = e.finding(input, "CONTROL_RESIDUE_DIFFERENCE", domain.SeverityBlocking, fmt.Sprintf("试验区与对照区残留偏差 %.3f 超过阈值", residueDifference), now)
		}
	} else if isTrialZone(input.Current.ZoneID, input.Controls) {
		violations["CONTROL_EVIDENCE_MISSING"] = e.finding(input, "CONTROL_EVIDENCE_MISSING", domain.SeverityBlocking, "缺少同轮次对照区观察，无法完成对照评估", now)
	}
	if trendRising(input.History, input.Current, func(o domain.TrialObservation) float64 { return o.ParticleLossScore }) {
		violations["PARTICLE_LOSS_TREND"] = e.finding(input, "PARTICLE_LOSS_TREND", domain.SeverityBlocking, "连续观察显示颗粒脱落呈上升趋势", now)
	}
	if input.Current.MoisturePercent > input.Baseline.MoisturePercent+input.Protocol.SafetyThresholds.MaxControlDifference {
		violations["BASELINE_MOISTURE_DEVIATION"] = e.finding(input, "BASELINE_MOISTURE_DEVIATION", domain.SeverityBlocking, "含水率相对病害基线增幅过大", now)
	}
	merged := mergeFindings(input.OpenFindings, violations, input.Current, now)
	sort.Slice(merged, func(i, j int) bool { return merged[i].RuleCode < merged[j].RuleCode })
	result := Result{Findings: merged}
	for _, finding := range merged {
		if finding.ProtocolRevisionID == input.Protocol.RevisionID && finding.Status == domain.FindingOpen && finding.Severity == domain.SeverityBlocking {
			result.HasBlockers = true
		}
	}
	result.Snapshot = domain.EvaluationSnapshot{
		SnapshotID: e.id("snapshot"), CaseID: input.Case.CaseID,
		ProtocolRevisionID: input.Protocol.RevisionID, ObservationID: input.Current.ObservationID,
		BaselineRevisionID: input.Baseline.RevisionID,
		Findings:           merged, EligibleForReview: !result.HasBlockers, EvaluatedAt: now,
		Metrics: buildMetricAssessments(input),
	}
	return result
}

func (e *Evaluator) finding(input Input, code string, severity domain.Severity, message string, now time.Time) domain.RiskFinding {
	return domain.RiskFinding{FindingID: e.id("finding"), CaseID: input.Case.CaseID, ProtocolRevisionID: input.Protocol.RevisionID, BaselineRevisionID: input.Baseline.RevisionID, RuleCode: code, Severity: severity, Message: message, EvidenceRefs: []string{input.Current.ObservationID, input.Current.EvidenceDigest}, Status: domain.FindingOpen, OpenedAt: now}
}

func mergeFindings(existing []domain.RiskFinding, current map[string]domain.RiskFinding, observation domain.TrialObservation, now time.Time) []domain.RiskFinding {
	result := make([]domain.RiskFinding, 0, len(existing)+len(current))
	seen := map[string]bool{}
	for _, old := range existing {
		if replacement, exists := current[old.RuleCode]; exists {
			if old.ProtocolRevisionID == observation.ProtocolRevisionID && old.BaselineRevisionID == observation.BaselineRevisionID {
				replacement.FindingID = old.FindingID
				replacement.OpenedAt = old.OpenedAt
				replacement.EvidenceRefs = append(old.EvidenceRefs, observation.ObservationID, observation.EvidenceDigest)
				result = append(result, replacement)
				seen[old.RuleCode] = true
				continue
			}
			result = append(result, old)
			continue
		}
		result = append(result, old)
		if old.ProtocolRevisionID == observation.ProtocolRevisionID && old.BaselineRevisionID == observation.BaselineRevisionID {
			seen[old.RuleCode] = true
		}
	}
	for code, finding := range current {
		if !seen[code] {
			result = append(result, finding)
		}
	}
	return result
}
