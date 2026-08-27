package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"mural-conservation-gate/internal/domain"
)

type checkClient struct {
	base   string
	client *http.Client
	serial int
}

func runSelfcheck(ctx context.Context, base string) error {
	c := &checkClient{base: base, client: &http.Client{Timeout: 5 * time.Second}}
	if err := c.health(ctx); err != nil {
		return err
	}
	var created domain.ConservationCase
	if err := c.post(ctx, "/api/cases", map[string]any{"siteName": "自检历史建筑", "muralLocation": "东壁自检区", "materialLayers": []string{"地仗层", "颜料层"}, "pathologies": []string{"烟熏", "粉化"}, "ambientCondition": map[string]any{"temperatureC": 21, "humidityPercent": 50, "lightLux": 80}, "actorId": "selfcheck-engineer"}, &created); err != nil {
		return err
	}
	caseID, version := created.CaseID, created.Version
	now := time.Now().UTC().Add(-time.Minute)
	view, err := c.casePost(ctx, caseID, "baseline", map[string]any{"expectedVersion": version, "colorL": 52, "colorA": 8, "colorB": 12, "moisturePercent": 4, "particleCondition": "表面稳定，局部粉化", "residueBaseline": 0.2, "measuredAt": now, "measuredBy": "selfcheck-engineer", "ambient": map[string]any{"temperatureC": 21, "humidityPercent": 50, "lightLux": 80}})
	if err != nil {
		return err
	}
	version = view.Case.Version
	view, err = c.casePost(ctx, caseID, "zones", map[string]any{"expectedVersion": version, "zoneType": "control", "boundaryPoints": []map[string]float64{{"x": 0, "y": 0}, {"x": 10, "y": 0}, {"x": 10, "y": 10}, {"x": 0, "y": 10}}, "areaCm2": 100, "representativeReason": "自检典型对照面", "controlZoneId": "", "actorId": "selfcheck-engineer"})
	if err != nil {
		return err
	}
	controlID, version := view.Zones[0].ZoneID, view.Case.Version
	view, err = c.casePost(ctx, caseID, "zones", map[string]any{"expectedVersion": version, "zoneType": "trial", "boundaryPoints": []map[string]float64{{"x": 20, "y": 0}, {"x": 30, "y": 0}, {"x": 30, "y": 10}, {"x": 20, "y": 10}}, "areaCm2": 100, "representativeReason": "自检典型试验面", "controlZoneId": controlID, "actorId": "selfcheck-engineer"})
	if err != nil {
		return err
	}
	trialID, version := view.Zones[1].ZoneID, view.Case.Version
	view, err = c.casePost(ctx, caseID, "protocols", map[string]any{"expectedVersion": version, "protocolId": "selfcheck-protocol", "ingredients": []map[string]any{{"name": "去离子水凝胶", "percentage": 95}}, "concentration": 5, "contactSeconds": 30, "tools": []string{"棉签", "吸水纸"}, "removalSteps": []string{"点涂", "计时", "吸除", "复测"}, "safetyThresholds": map[string]any{"maxColorDeltaE": 3, "maxParticleLossScore": 1.5, "maxMoisturePercent": 10, "maxResidueValue": 2, "maxControlDifference": 2}, "changeReason": "自检首版", "createdBy": "selfcheck-engineer"})
	if err != nil {
		return err
	}
	revisionID, version := view.Protocols[0].RevisionID, view.Case.Version
	controlObservation := map[string]any{"zoneId": controlID, "protocolRevisionId": revisionID, "roundNo": 1, "observedAt": now, "colorDeltaE": 0.3, "particleLossScore": 0.1, "moisturePercent": 4.1, "residueValue": 0.2, "evidenceDigest": "selfcheck-control-evidence", "evidenceSummary": "对照区同轮复测证据", "operatorId": "selfcheck-engineer"}
	trialObservation := map[string]any{"zoneId": trialID, "protocolRevisionId": revisionID, "roundNo": 1, "observedAt": now, "colorDeltaE": 1.1, "particleLossScore": 0.4, "moisturePercent": 4.5, "residueValue": 0.6, "evidenceDigest": "selfcheck-trial-evidence", "evidenceSummary": "试验区清洗前后复测证据", "operatorId": "selfcheck-engineer"}
	view, err = c.casePost(ctx, caseID, "observation-batches", map[string]any{"expectedVersion": version, "control": controlObservation, "trials": []any{trialObservation}})
	if err != nil {
		return err
	}
	if view.Case.Status != domain.StatusReview {
		return fmt.Errorf("自检风险评估未进入待复核状态: %s", view.Case.Status)
	}
	view, err = c.casePost(ctx, caseID, "candidate-selection", map[string]any{"expectedVersion": view.Case.Version, "protocolRevisionId": revisionID, "reviewerId": "selfcheck-reviewer", "reason": "自检横向比较确认当前候选证据完整且阈值裕量合格"})
	if err != nil {
		return err
	}
	view, err = c.casePost(ctx, caseID, "freeze", map[string]any{"expectedVersion": view.Case.Version, "protocolRevisionId": revisionID, "reviewerId": "selfcheck-reviewer", "reviewNote": "自检证据完整且风险闭环"})
	if err != nil {
		return err
	}
	view, err = c.casePost(ctx, caseID, "permit", map[string]any{"expectedVersion": view.Case.Version, "scope": "自检试验验证范围", "restrictions": []string{"保持冻结参数"}, "issuedBy": "selfcheck-manager", "expiresAt": time.Now().UTC().Add(24 * time.Hour)})
	if err != nil {
		return err
	}
	if view.Permit == nil {
		return fmt.Errorf("自检未生成许可")
	}
	var verification struct {
		Valid bool `json:"valid"`
	}
	if err = c.get(ctx, "/api/permits/"+view.Permit.PermitNumber+"/verify", &verification); err != nil {
		return err
	}
	if !verification.Valid {
		return fmt.Errorf("自检许可验真失败")
	}
	var audit struct {
		Events []domain.AuditEvent `json:"events"`
	}
	if err = c.get(ctx, "/api/cases/"+caseID+"/audit", &audit); err != nil {
		return err
	}
	if len(audit.Events) < 9 {
		return fmt.Errorf("自检审计事件不完整: %d", len(audit.Events))
	}
	return nil
}

func (c *checkClient) health(ctx context.Context) error {
	var result map[string]any
	return c.get(ctx, "/healthz", &result)
}

func (c *checkClient) casePost(ctx context.Context, caseID, action string, payload any) (domain.CaseView, error) {
	var view domain.CaseView
	err := c.post(ctx, "/api/cases/"+caseID+"/"+action, payload, &view)
	return view, err
}

func (c *checkClient) post(ctx context.Context, path string, payload, target any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	c.serial++
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", fmt.Sprintf("selfcheck-%d", c.serial))
	return c.do(req, target)
}

func (c *checkClient) get(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, target)
}

func (c *checkClient) do(req *http.Request, target any) error {
	response, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%s %s 返回 %d: %s", req.Method, req.URL.Path, response.StatusCode, data)
	}
	if err = json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("解析 %s: %w", req.URL.Path, err)
	}
	return nil
}
