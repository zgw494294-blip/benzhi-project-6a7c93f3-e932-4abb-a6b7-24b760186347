package evaluation

import (
	"fmt"
	"testing"
	"time"

	"mural-conservation-gate/internal/domain"
)

func TestEvaluatorCreatesStableRules(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	serial := 0
	e := New(func() time.Time { return now }, func(prefix string) string { serial++; return fmt.Sprintf("%s-%d", prefix, serial) })
	protocol := domain.CleaningProtocolRevision{RevisionID: "rev", SafetyThresholds: domain.SafetyThresholds{MaxColorDeltaE: 3, MaxParticleLossScore: 1, MaxMoisturePercent: 10, MaxResidueValue: 2, MaxControlDifference: 2}}
	current := domain.TrialObservation{ObservationID: "trial", ZoneID: "trial-zone", ProtocolRevisionID: "rev", RoundNo: 1, ColorDeltaE: 5, ParticleLossScore: .2, MoisturePercent: 4, ResidueValue: .5, EvidenceDigest: "digest"}
	control := domain.TrialObservation{ObservationID: "control", ZoneID: "control-zone", ProtocolRevisionID: "rev", RoundNo: 1, ColorDeltaE: .2, ResidueValue: .1}
	result := e.Evaluate(Input{Case: domain.ConservationCase{CaseID: "case"}, Baseline: domain.BaselineRevision{MoisturePercent: 4}, Protocol: protocol, Current: current, Controls: []domain.TrialObservation{control}})
	if !result.HasBlockers {
		t.Fatal("超阈值色差必须产生阻断")
	}
	codes := map[string]bool{}
	for _, finding := range result.Findings {
		codes[finding.RuleCode] = true
	}
	if !codes["COLOR_DELTA_LIMIT"] || !codes["CONTROL_COLOR_DIFFERENCE"] {
		t.Fatalf("风险规则不完整: %#v", codes)
	}
}
