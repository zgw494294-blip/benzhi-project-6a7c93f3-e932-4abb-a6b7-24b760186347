package evaluation

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"mural-conservation-gate/internal/domain"
)

func CompareCandidates(view domain.CaseView) domain.CandidateComparisonSet {
	latest := domain.LatestProtocolRevisions(view.Protocols)
	result := domain.CandidateComparisonSet{Candidates: []domain.CandidateComparison{}}
	for _, protocol := range latest {
		result.Candidates = append(result.Candidates, compareCandidate(view, protocol))
	}
	data, _ := json.Marshal(result.Candidates)
	result.Digest = domain.DigestBytes(data)
	return result
}

func compareCandidate(view domain.CaseView, protocol domain.CleaningProtocolRevision) domain.CandidateComparison {
	item := domain.CandidateComparison{ProtocolRevisionID: protocol.RevisionID, ProtocolID: protocol.ProtocolID, RevisionNo: protocol.RevisionNo, Metrics: []domain.CandidateMetric{}, Gaps: []string{}, TrendConclusion: "无恶化趋势"}
	trials := []domain.TrialZone{}
	for _, z := range view.Zones {
		if z.ZoneType == domain.ZoneTrial {
			trials = append(trials, z)
		}
	}
	item.RequiredPairs = len(trials)
	observations := map[string]map[int]domain.TrialObservation{}
	for _, o := range view.Observations {
		if o.ProtocolRevisionID != protocol.RevisionID || o.BaselineRevisionID != view.Case.BaselineRevisionID {
			continue
		}
		if observations[o.ZoneID] == nil {
			observations[o.ZoneID] = map[int]domain.TrialObservation{}
		}
		observations[o.ZoneID][o.RoundNo] = o
	}
	covered := map[string]bool{}
	for _, z := range trials {
		for round := range observations[z.ZoneID] {
			if _, ok := observations[z.ControlZoneID][round]; ok {
				covered[z.ZoneID] = true
			}
		}
	}
	item.CoveredPairs = len(covered)
	if item.RequiredPairs > 0 {
		item.CoveragePercent = math.Round(float64(item.CoveredPairs)*1000/float64(item.RequiredPairs)) / 10
	}
	if item.CoveredPairs < item.RequiredPairs {
		item.Gaps = append(item.Gaps, fmt.Sprintf("缺少 %d 个试验区的当前基线同轮对照证据", item.RequiredPairs-item.CoveredPairs))
	}
	worst := map[string]float64{}
	worstControl := map[string]float64{}
	passed := map[string]bool{}
	for _, snapshot := range view.Snapshots {
		if snapshot.ProtocolRevisionID != protocol.RevisionID || snapshot.BaselineRevisionID != view.Case.BaselineRevisionID {
			continue
		}
		for _, m := range snapshot.Metrics {
			if _, ok := worst[m.Code]; !ok || m.ThresholdMargin < worst[m.Code] {
				worst[m.Code] = m.ThresholdMargin
			}
			if _, ok := passed[m.Code]; !ok {
				passed[m.Code] = true
			}
			passed[m.Code] = passed[m.Code] && m.Passed
			if m.ControlDifference != nil && math.Abs(*m.ControlDifference) > worstControl[m.Code] {
				worstControl[m.Code] = math.Abs(*m.ControlDifference)
			}
			if m.TrendPercent != nil && *m.TrendPercent > 0 {
				item.TrendConclusion = "存在指标上升趋势"
			}
		}
	}
	codes := make([]string, 0, len(worst))
	for code := range worst {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		item.Metrics = append(item.Metrics, domain.CandidateMetric{Code: code, WorstMargin: worst[code], WorstControlDifference: worstControl[code], Passed: passed[code]})
	}
	for _, f := range view.Findings {
		if !f.Historical && f.ProtocolRevisionID == protocol.RevisionID && f.Status == domain.FindingOpen && f.Severity == domain.SeverityBlocking {
			item.OpenBlockers++
		}
	}
	if item.OpenBlockers > 0 {
		item.Gaps = append(item.Gaps, fmt.Sprintf("仍有 %d 个开放阻断项", item.OpenBlockers))
	}
	item.Eligible = item.RequiredPairs > 0 && item.CoveredPairs == item.RequiredPairs && item.OpenBlockers == 0 && len(item.Metrics) > 0
	relevantObservations := []domain.TrialObservation{}
	for _, observation := range view.Observations {
		if observation.ProtocolRevisionID == protocol.RevisionID && observation.BaselineRevisionID == view.Case.BaselineRevisionID {
			relevantObservations = append(relevantObservations, observation)
		}
	}
	relevantSnapshots := []domain.EvaluationSnapshot{}
	for _, snapshot := range view.Snapshots {
		if snapshot.ProtocolRevisionID == protocol.RevisionID && snapshot.BaselineRevisionID == view.Case.BaselineRevisionID {
			relevantSnapshots = append(relevantSnapshots, snapshot)
		}
	}
	relevantFindings := []domain.RiskFinding{}
	for _, finding := range view.Findings {
		if finding.ProtocolRevisionID == protocol.RevisionID && !finding.Historical {
			relevantFindings = append(relevantFindings, finding)
		}
	}
	sort.Slice(relevantObservations, func(i, j int) bool {
		return relevantObservations[i].ObservationID < relevantObservations[j].ObservationID
	})
	sort.Slice(relevantSnapshots, func(i, j int) bool { return relevantSnapshots[i].SnapshotID < relevantSnapshots[j].SnapshotID })
	sort.Slice(relevantFindings, func(i, j int) bool { return relevantFindings[i].FindingID < relevantFindings[j].FindingID })
	facts, _ := json.Marshal(struct {
		Protocol           domain.CleaningProtocolRevision `json:"protocol"`
		BaselineRevisionID string                          `json:"baselineRevisionId"`
		Observations       []domain.TrialObservation       `json:"observations"`
		Snapshots          []domain.EvaluationSnapshot     `json:"snapshots"`
		Findings           []domain.RiskFinding            `json:"findings"`
	}{protocol, view.Case.BaselineRevisionID, relevantObservations, relevantSnapshots, relevantFindings})
	item.FactsDigest = domain.DigestBytes(facts)
	canonical := item
	canonical.Digest = ""
	data, _ := json.Marshal(canonical)
	item.Digest = domain.DigestBytes(data)
	return item
}
