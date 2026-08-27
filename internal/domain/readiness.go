package domain

import (
	"fmt"
	"sort"
)

type ReadinessCheck struct {
	Code   string `json:"code"`
	Label  string `json:"label"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type WorkflowReadiness struct {
	CanDesign      bool             `json:"canDesign"`
	CanObserve     bool             `json:"canObserve"`
	CanFreeze      bool             `json:"canFreeze"`
	CanIssuePermit bool             `json:"canIssuePermit"`
	Checks         []ReadinessCheck `json:"checks"`
}

func AssessReadiness(view CaseView) WorkflowReadiness {
	result := WorkflowReadiness{Checks: []ReadinessCheck{}}
	baselineReady := view.Baseline != nil && view.Case.BaselineRevisionID != ""
	result.Checks = append(result.Checks, ReadinessCheck{Code: "BASELINE_COMPLETE", Label: "病害基线完整", Passed: baselineReady, Detail: choose(baselineReady, "已有版本化基线测量", "需要提交颜色、含水率、颗粒和残留基线")})
	hasTrial, hasControl, allPaired := false, false, true
	zoneTypes := map[string]ZoneType{}
	for _, zone := range view.Zones {
		zoneTypes[zone.ZoneID] = zone.ZoneType
	}
	for _, zone := range view.Zones {
		hasTrial = hasTrial || zone.ZoneType == ZoneTrial
		hasControl = hasControl || zone.ZoneType == ZoneControl
		if zone.ZoneType == ZoneTrial && zoneTypes[zone.ControlZoneID] != ZoneControl {
			allPaired = false
		}
	}
	zonesReady := hasTrial && hasControl && allPaired
	result.Checks = append(result.Checks, ReadinessCheck{Code: "ZONES_PAIRED", Label: "试验与对照分区成对", Passed: zonesReady, Detail: choose(zonesReady, "试验区均关联有效对照区", "至少需要一个试验区、一个对照区及有效关联")})
	latest := LatestProtocolRevisions(view.Protocols)
	protocolReady := len(latest) > 0
	result.Checks = append(result.Checks, ReadinessCheck{Code: "PROTOCOL_AVAILABLE", Label: "候选方案可评估", Passed: protocolReady, Detail: choose(protocolReady, "已有不可变候选方案修订", "需要提交材料、参数、步骤和安全阈值")})
	evidenceReady := evidenceCoversZones(view, latest)
	result.Checks = append(result.Checks, ReadinessCheck{Code: "EVIDENCE_COVERAGE", Label: "试验与对照证据完整", Passed: evidenceReady, Detail: choose(evidenceReady, "最新方案具有试验和对照观察", "最新方案仍缺少成对观察证据")})
	open := 0
	for _, finding := range view.Findings {
		if !finding.Historical && finding.Status == FindingOpen && finding.Severity == SeverityBlocking {
			open++
		}
	}
	risksClosed := open == 0
	result.Checks = append(result.Checks, ReadinessCheck{Code: "RISKS_CLOSED", Label: "风险阻断全部闭环", Passed: risksClosed, Detail: choose(risksClosed, "没有开放的阻断项", riskCountDetail(open))})
	frozen := view.Manifest != nil && (view.Case.Status == StatusFrozen || view.Case.Status == StatusPermitted)
	result.Checks = append(result.Checks, ReadinessCheck{Code: "MANIFEST_FROZEN", Label: "证据清单已冻结", Passed: frozen, Detail: choose(frozen, "方案版本和证据摘要不可变", "等待复核员批准唯一候选方案")})
	result.CanDesign = baselineReady && view.Case.Status != StatusFrozen && view.Case.Status != StatusPermitted
	result.CanObserve = result.CanDesign && zonesReady && protocolReady
	selected := view.Selection != nil && view.Selection.Valid
	result.Checks = append(result.Checks, ReadinessCheck{Code: "CANDIDATE_SELECTED", Label: "唯一候选方案已入围", Passed: selected, Detail: choose(selected, "比选摘要有效且唯一入围", "需要对最新候选方案完成横向比选并指定唯一入围修订")})
	result.CanFreeze = view.Case.Status == StatusReview && evidenceReady && risksClosed && selected
	result.CanIssuePermit = view.Case.Status == StatusFrozen && view.Manifest != nil
	return result
}

func LatestProtocolRevisions(values []CleaningProtocolRevision) []CleaningProtocolRevision {
	latest := map[string]CleaningProtocolRevision{}
	for _, value := range values {
		current, exists := latest[value.ProtocolID]
		if !exists || value.RevisionNo > current.RevisionNo {
			latest[value.ProtocolID] = value
		}
	}
	result := make([]CleaningProtocolRevision, 0, len(latest))
	for _, value := range latest {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ProtocolID < result[j].ProtocolID })
	return result
}

func evidenceCoversZones(view CaseView, protocols []CleaningProtocolRevision) bool {
	if len(protocols) == 0 {
		return false
	}
	latestIDs := map[string]bool{}
	for _, protocol := range protocols {
		latestIDs[protocol.RevisionID] = true
	}
	zoneTypes := map[string]ZoneType{}
	for _, zone := range view.Zones {
		zoneTypes[zone.ZoneID] = zone.ZoneType
	}
	pairs := map[string]map[int]bool{}
	for _, observation := range view.Observations {
		if !latestIDs[observation.ProtocolRevisionID] || observation.BaselineRevisionID != view.Case.BaselineRevisionID {
			continue
		}
		key := observation.ProtocolRevisionID + "\x00" + observation.ZoneID
		if pairs[key] == nil {
			pairs[key] = map[int]bool{}
		}
		pairs[key][observation.RoundNo] = true
	}
	trialCount := 0
	for _, zone := range view.Zones {
		if zone.ZoneType != ZoneTrial {
			continue
		}
		trialCount++
		matched := false
		for _, protocol := range protocols {
			for round := range pairs[protocol.RevisionID+"\x00"+zone.ZoneID] {
				if pairs[protocol.RevisionID+"\x00"+zone.ControlZoneID][round] {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}
	return trialCount > 0
}

func choose(condition bool, yes, no string) string {
	if condition {
		return yes
	}
	return no
}

func riskCountDetail(count int) string {
	if count == 1 {
		return "仍有 1 个开放阻断项"
	}
	return fmt.Sprintf("仍有 %d 个开放阻断项", count)
}
