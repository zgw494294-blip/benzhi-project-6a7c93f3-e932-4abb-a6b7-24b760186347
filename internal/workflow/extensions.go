package workflow

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"mural-conservation-gate/internal/domain"
	"mural-conservation-gate/internal/evaluation"
	"mural-conservation-gate/internal/store"
)

type ListCasesQuery struct {
	Status          domain.CaseStatus
	Keyword         string
	KeywordProvided bool
	Cursor          string
	Limit           int
}

type queueCursor struct {
	UpdatedAt time.Time         `json:"updatedAt"`
	CaseID    string            `json:"caseId"`
	Status    domain.CaseStatus `json:"status,omitempty"`
	Keyword   string            `json:"keyword,omitempty"`
}

func (s *Service) ListCases(ctx context.Context, query ListCasesQuery) (domain.CaseQueue, error) {
	known := map[domain.CaseStatus]bool{domain.StatusDraft: true, domain.StatusTesting: true, domain.StatusRemediation: true, domain.StatusReview: true, domain.StatusFrozen: true, domain.StatusPermitted: true}
	if query.Status != "" && !known[query.Status] {
		return domain.CaseQueue{}, domain.Invalid("status", "未知案卷状态")
	}
	query.Keyword = strings.TrimSpace(query.Keyword)
	if query.KeywordProvided && query.Keyword == "" {
		return domain.CaseQueue{}, domain.Invalid("keyword", "关键词不得为空白")
	}
	if len([]rune(query.Keyword)) > 100 {
		return domain.CaseQueue{}, domain.Invalid("keyword", "关键词长度不得超过 100 个字符")
	}
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Limit < 1 || query.Limit > 100 {
		return domain.CaseQueue{}, domain.Invalid("limit", "分页大小必须在 1 到 100 之间")
	}
	search := store.CaseSearch{Status: query.Status, Keyword: query.Keyword, Limit: query.Limit + 1}
	if query.Cursor != "" {
		data, err := base64.RawURLEncoding.DecodeString(query.Cursor)
		if err != nil {
			return domain.CaseQueue{}, domain.Invalid("cursor", "分页游标无效")
		}
		var cursor queueCursor
		if json.Unmarshal(data, &cursor) != nil || cursor.UpdatedAt.IsZero() || strings.TrimSpace(cursor.CaseID) == "" || cursor.Status != query.Status || cursor.Keyword != query.Keyword {
			return domain.CaseQueue{}, domain.Invalid("cursor", "分页游标无效")
		}
		search.CursorUpdatedAt = &cursor.UpdatedAt
		search.CursorCaseID = cursor.CaseID
	}
	result, err := s.store.SearchCases(ctx, search)
	if err != nil {
		return domain.CaseQueue{}, err
	}
	hasMore := len(result.Cases) > query.Limit
	if hasMore {
		result.Cases = result.Cases[:query.Limit]
	}
	queue := domain.CaseQueue{Items: []domain.CaseQueueSummary{}, Total: result.Total, StatusCounts: result.StatusCounts}
	for status := range known {
		if _, ok := queue.StatusCounts[status]; !ok {
			queue.StatusCounts[status] = 0
		}
	}
	for _, c := range result.Cases {
		view, loadErr := s.GetCase(ctx, c.CaseID)
		if loadErr != nil {
			return queue, loadErr
		}
		summary := domain.CaseQueueSummary{Case: view.Case, Readiness: view.Readiness, MissingEvidence: domain.MissingEvidence(view.Readiness)}
		if view.Baseline != nil {
			summary.CurrentRevision = view.Baseline.RevisionNo
		}
		for _, f := range view.Findings {
			if !f.Historical && f.Status == domain.FindingOpen && f.Severity == domain.SeverityBlocking {
				summary.OpenBlockers++
			}
		}
		if view.Permit != nil {
			expiry := view.Permit.ExpiresAt
			summary.PermitExpiresAt = &expiry
			summary.PermitValidity = permitValidity(*view.Permit, s.now())
			if view.Manifest == nil || domain.PermitVerificationDigest(*view.Permit) != view.Permit.VerificationDigest || view.Permit.EvidenceManifestDigest != view.Manifest.Digest || view.Permit.FrozenProtocolRevisionID != view.Manifest.ProtocolRevisionID {
				summary.PermitValidity = domain.PermitMismatch
			}
		}
		queue.Items = append(queue.Items, summary)
	}
	if hasMore && len(queue.Items) > 0 {
		last := queue.Items[len(queue.Items)-1].Case
		data, _ := json.Marshal(queueCursor{UpdatedAt: last.UpdatedAt, CaseID: last.CaseID, Status: query.Status, Keyword: query.Keyword})
		queue.NextCursor = base64.RawURLEncoding.EncodeToString(data)
	}
	return queue, nil
}

func permitValidity(permit domain.WorkPermit, now time.Time) domain.PermitValidity {
	if !now.Before(permit.ExpiresAt) {
		return domain.PermitExpired
	}
	if permit.ExpiresAt.Sub(now) <= 7*24*time.Hour {
		return domain.PermitExpiringSoon
	}
	return domain.PermitValid
}

func (s *Service) SelectCandidate(ctx context.Context, caseID string, cmd SelectionCommand) (domain.CaseView, error) {
	return s.mutate(ctx, caseID, cmd.ExpectedVersion, func(tx *store.Tx, view domain.CaseView) (domain.CaseStatus, string, error) {
		if err := ensureState(view.Case, domain.StatusTesting, domain.StatusRemediation, domain.StatusReview); err != nil {
			return "", "", err
		}
		if strings.TrimSpace(cmd.Reason) == "" {
			return "", "", domain.Invalid("reason", "必须填写候选方案比选理由")
		}
		if strings.TrimSpace(cmd.ReviewerID) == "" {
			return "", "", domain.Invalid("reviewerId", "必须登记复核员")
		}
		comparison := evaluation.CompareCandidates(view)
		var chosen *domain.CandidateComparison
		for i := range comparison.Candidates {
			if comparison.Candidates[i].ProtocolRevisionID == cmd.ProtocolRevisionID {
				chosen = &comparison.Candidates[i]
				break
			}
		}
		if chosen == nil {
			return "", "", domain.Invalid("protocolRevisionId", "候选方案最新修订不存在")
		}
		if !chosen.Eligible {
			return "", "", domain.Invalid("protocolRevisionId", "候选方案证据不完整或仍有开放阻断项，不能入围")
		}
		selection := domain.CandidateSelection{SelectionID: newID("selection"), CaseID: caseID, ProtocolRevisionID: cmd.ProtocolRevisionID, ComparisonDigest: comparison.Digest, Reason: strings.TrimSpace(cmd.Reason), ReviewerID: strings.TrimSpace(cmd.ReviewerID), SelectedAt: s.now(), Valid: true}
		if err := tx.InsertSelection(ctx, selection); err != nil {
			return "", "", err
		}
		if err := tx.AppendAudit(ctx, caseID, "candidate.selected", selection.ReviewerID, "完成候选方案横向比选并指定唯一入围修订", selection); err != nil {
			return "", "", err
		}
		return domain.StatusReview, "", nil
	})
}

func (s *Service) CreateRemediation(ctx context.Context, caseID string, cmd RemediationCommand) (domain.CaseView, error) {
	return s.mutate(ctx, caseID, cmd.ExpectedVersion, func(tx *store.Tx, view domain.CaseView) (domain.CaseStatus, string, error) {
		if err := ensureState(view.Case, domain.StatusTesting, domain.StatusRemediation, domain.StatusReview); err != nil {
			return "", "", err
		}
		var finding *domain.RiskFinding
		for i := range view.Findings {
			if view.Findings[i].FindingID == cmd.FindingID {
				finding = &view.Findings[i]
				break
			}
		}
		if finding == nil {
			return "", "", domain.Invalid("findingId", "风险发现不存在或不属于当前案卷")
		}
		if finding.Status != domain.FindingOpen || finding.Historical {
			return "", "", domain.Invalid("findingId", "已关闭风险不能登记整改任务")
		}
		if strings.TrimSpace(cmd.Assignee) == "" {
			return "", "", domain.Invalid("assignee", "必须登记整改责任人")
		}
		if strings.TrimSpace(cmd.Analysis) == "" {
			return "", "", domain.Invalid("analysis", "必须填写问题分析")
		}
		if strings.TrimSpace(cmd.ActorID) == "" {
			return "", "", domain.Invalid("actorId", "必须登记操作人")
		}
		if !cmd.DueAt.After(s.now()) {
			return "", "", domain.Invalid("dueAt", "最晚完成时间必须晚于当前时间")
		}
		if cmd.PlannedParameters.Concentration == nil && cmd.PlannedParameters.ContactSeconds == nil {
			return "", "", domain.Invalid("plannedParameters", "至少登记一项计划调整参数")
		}
		if cmd.PlannedParameters.Concentration != nil && (*cmd.PlannedParameters.Concentration <= 0 || *cmd.PlannedParameters.Concentration > 100) {
			return "", "", domain.Invalid("plannedParameters.concentration", "计划浓度必须大于 0 且不超过 100")
		}
		if cmd.PlannedParameters.ContactSeconds != nil && (*cmd.PlannedParameters.ContactSeconds < 1 || *cmd.PlannedParameters.ContactSeconds > 86400) {
			return "", "", domain.Invalid("plannedParameters.contactSeconds", "计划接触时长必须在 1 秒到 24 小时之间")
		}
		sourceProtocol, sourceFound := findProtocol(view, finding.ProtocolRevisionID)
		if !sourceFound {
			return "", "", domain.Invalid("findingId", "风险关联的方案修订不存在")
		}
		changed := false
		if cmd.PlannedParameters.Concentration != nil && *cmd.PlannedParameters.Concentration != sourceProtocol.Concentration {
			changed = true
		}
		if cmd.PlannedParameters.ContactSeconds != nil && *cmd.PlannedParameters.ContactSeconds != sourceProtocol.ContactSeconds {
			changed = true
		}
		if !changed {
			return "", "", domain.Invalid("plannedParameters", "计划参数必须相对风险方案发生实际变化")
		}
		if len(cmd.RequiredZoneIDs) == 0 {
			return "", "", domain.Invalid("requiredZoneIds", "至少指定一个要求复验的试验区")
		}
		seen := map[string]bool{}
		for _, id := range cmd.RequiredZoneIDs {
			if seen[id] {
				return "", "", domain.Invalid("requiredZoneIds", "要求复验的分区不得重复")
			}
			seen[id] = true
			z, ok := findZone(view, id)
			if !ok || z.ZoneType != domain.ZoneTrial {
				return "", "", domain.Invalid("requiredZoneIds", "要求复验的分区必须是当前案卷试验区")
			}
		}
		var existingTask *domain.RemediationTask
		for i := range view.Remediations {
			existing := &view.Remediations[i]
			if existing.FindingID == finding.FindingID {
				existingTask = existing
				break
			}
		}
		now := s.now()
		task := domain.RemediationTask{TaskID: newID("remediation"), CaseID: caseID, FindingID: finding.FindingID, RuleCode: finding.RuleCode, SourceProtocolRevisionID: finding.ProtocolRevisionID, Assignee: strings.TrimSpace(cmd.Assignee), Analysis: strings.TrimSpace(cmd.Analysis), PlannedParameters: cmd.PlannedParameters, RequiredZoneIDs: append([]string(nil), cmd.RequiredZoneIDs...), DueAt: cmd.DueAt, Status: domain.RemediationPending, CreatedAt: now, UpdatedAt: now}
		eventType, eventSummary := "remediation.created", "为开放风险登记责任人、参数目标和成对复验要求"
		if existingTask != nil {
			if existingTask.Status != domain.RemediationPending {
				return "", "", domain.Invalid("findingId", "整改任务进入待复验后不可改写计划")
			}
			task.TaskID = existingTask.TaskID
			task.CreatedAt = existingTask.CreatedAt
			task.SourceProtocolRevisionID = existingTask.SourceProtocolRevisionID
			task.RuleCode = existingTask.RuleCode
			eventType, eventSummary = "remediation.updated", "更新待执行整改任务的责任人、参数目标和复验要求"
		}
		if err := tx.UpsertRemediation(ctx, task, finding.Severity); err != nil {
			return "", "", err
		}
		if err := tx.AppendAudit(ctx, caseID, eventType, cmd.ActorID, eventSummary, task); err != nil {
			return "", "", err
		}
		return domain.StatusRemediation, "", nil
	})
}

func (s *Service) RemediationQueue(ctx context.Context, assignee string, overdue bool, severity domain.Severity) ([]domain.RemediationTask, error) {
	assignee = strings.TrimSpace(assignee)
	if severity != "" && severity != domain.SeverityWarning && severity != domain.SeverityBlocking {
		return nil, domain.Invalid("severity", "未知风险严重度")
	}
	return s.store.RemediationQueue(ctx, assignee, overdue, severity, s.now())
}

func (s *Service) SubmitPairedObservations(ctx context.Context, caseID string, cmd PairedObservationBatchCommand) (domain.CaseView, error) {
	return s.mutate(ctx, caseID, cmd.ExpectedVersion, func(tx *store.Tx, view domain.CaseView) (domain.CaseStatus, string, error) {
		if err := ensureState(view.Case, domain.StatusTesting, domain.StatusRemediation, domain.StatusReview); err != nil {
			return "", "", err
		}
		if view.Baseline == nil {
			return "", "", fmt.Errorf("%w: 未提交病害基线", domain.ErrIncomplete)
		}
		if len(cmd.Trials) == 0 {
			return "", "", domain.Invalid("trials", "至少提交一个关联试验区观察")
		}
		controlZone, ok := findZone(view, cmd.Control.ZoneID)
		if !ok || controlZone.ZoneType != domain.ZoneControl {
			return "", "", domain.Invalid("control.zoneId", "批次对照观察必须引用有效对照区")
		}
		protocol, ok := findProtocol(view, cmd.Control.ProtocolRevisionID)
		if !ok {
			return "", "", domain.Invalid("control.protocolRevisionId", "方案修订不存在")
		}
		inputs := append([]ObservationInput{cmd.Control}, cmd.Trials...)
		observations := make([]domain.TrialObservation, 0, len(inputs))
		seenZones := map[string]bool{}
		earliest, latest := inputs[0].ObservedAt, inputs[0].ObservedAt
		for i, input := range inputs {
			field := "trials"
			if i == 0 {
				field = "control"
			}
			if input.ProtocolRevisionID != protocol.RevisionID {
				return "", "", domain.Invalid(field+".protocolRevisionId", "批次内必须使用同一方案修订")
			}
			if input.RoundNo != cmd.Control.RoundNo {
				return "", "", domain.Invalid(field+".roundNo", "批次内必须使用同一轮次")
			}
			zone, exists := findZone(view, input.ZoneID)
			if !exists {
				return "", "", domain.Invalid(field+".zoneId", "分区不存在")
			}
			if i > 0 && (zone.ZoneType != domain.ZoneTrial || zone.ControlZoneID != controlZone.ZoneID) {
				return "", "", domain.Invalid(field+".zoneId", "试验区未关联请求中的对照区")
			}
			if seenZones[input.ZoneID] {
				return "", "", domain.Invalid(field+".zoneId", "批次内分区不得重复")
			}
			seenZones[input.ZoneID] = true
			observation := observationFromInput(caseID, view.Case.BaselineRevisionID, input, s.now())
			if err := domain.ValidateObservation(observation); err != nil {
				return "", "", err
			}
			if observation.ObservedAt.Before(view.Baseline.MeasuredAt) {
				return "", "", domain.Invalid(field+".observedAt", "观察时间不得早于当前基线")
			}
			if observation.ObservedAt.Before(protocol.CreatedAt.Add(-5 * time.Minute)) {
				return "", "", domain.Invalid(field+".observedAt", "观察时间不得早于方案修订")
			}
			if observation.ObservedAt.Before(earliest) {
				earliest = observation.ObservedAt
			}
			if observation.ObservedAt.After(latest) {
				latest = observation.ObservedAt
			}
			for _, old := range view.Observations {
				if old.ZoneID == observation.ZoneID && old.ProtocolRevisionID == observation.ProtocolRevisionID && old.RoundNo == observation.RoundNo {
					return "", "", domain.Invalid(field+".roundNo", "该分区、方案修订和轮次已经存在")
				}
			}
			observations = append(observations, observation)
		}
		if latest.Sub(earliest) > 24*time.Hour {
			return "", "", domain.Invalid("observedAt", "成对观察必须处于同一 24 小时现场时间窗口")
		}
		for _, observation := range observations {
			if err := tx.InsertObservation(ctx, observation); err != nil {
				return "", "", normalizeStoreError(err, "roundNo")
			}
		}
		workingFindings := currentOpenFindings(view.Findings)
		history := append([]domain.TrialObservation(nil), view.Observations...)
		history = append(history, observations[0])
		snapshots := map[string]domain.EvaluationSnapshot{}
		hasCurrentBlockers := false
		results := make([]evaluation.Result, len(observations)-1)
		start := make(chan struct{})
		var ready sync.WaitGroup
		var finished sync.WaitGroup
		ready.Add(len(results))
		finished.Add(len(results))
		for i, observation := range observations[1:] {
			go func(index int, current domain.TrialObservation) {
				defer finished.Done()
				ready.Done()
				<-start
				result := s.eval.Evaluate(evaluation.Input{Case: view.Case, Baseline: *view.Baseline, Protocol: protocol, Current: current, History: history, Controls: []domain.TrialObservation{observations[0]}, OpenFindings: workingFindings})
				workingFindings = result.Findings
				history = append(history, current)
				hasCurrentBlockers = hasCurrentBlockers || result.HasBlockers
				results[index] = result
			}(i, observation)
		}
		ready.Wait()
		close(start)
		finished.Wait()
		for i, result := range results {
			for _, finding := range result.Findings {
				if err := tx.UpsertFinding(ctx, finding); err != nil {
					return "", "", err
				}
			}
			if err := tx.InsertSnapshot(ctx, result.Snapshot); err != nil {
				return "", "", err
			}
			snapshots[observations[i+1].ZoneID] = result.Snapshot
		}
		resolved := map[string]bool{}
		for _, task := range view.Remediations {
			if task.Status != domain.RemediationRetest || task.RetestProtocolRevisionID != protocol.RevisionID {
				continue
			}
			observationIDs, snapshotIDs := []string{}, []string{}
			passed := true
			for _, zoneID := range task.RequiredZoneIDs {
				snapshot, exists := snapshots[zoneID]
				if !exists || snapshotHasRule(snapshot, protocol.RevisionID, task.RuleCode) {
					passed = false
					break
				}
				snapshotIDs = append(snapshotIDs, snapshot.SnapshotID)
				observationIDs = append(observationIDs, snapshot.ObservationID)
			}
			if !passed {
				continue
			}
			task.Status = domain.RemediationResolved
			task.ResolvedObservationIDs = observationIDs
			task.ResolvedSnapshotIDs = snapshotIDs
			task.UpdatedAt = s.now()
			for _, finding := range view.Findings {
				if finding.FindingID == task.FindingID {
					now := s.now()
					finding.Status = domain.FindingClosed
					finding.RemediationNote = "整改计划指定分区在新方案下完成成对复验且目标规则通过"
					finding.ResolvedByObservationID = strings.Join(observationIDs, ",")
					finding.ClosedAt = &now
					if err := tx.UpsertFinding(ctx, finding); err != nil {
						return "", "", err
					}
					if err := tx.UpsertRemediation(ctx, task, finding.Severity); err != nil {
						return "", "", err
					}
					resolved[finding.FindingID] = true
				}
			}
			if err := tx.AppendAudit(ctx, caseID, "remediation.resolved", observations[len(observations)-1].OperatorID, "目标规则经指定分区成对复验通过，系统自动证据销项", task); err != nil {
				return "", "", err
			}
		}
		remaining := hasCurrentBlockers
		for _, finding := range view.Findings {
			if !finding.Historical && finding.Status == domain.FindingOpen && finding.Severity == domain.SeverityBlocking && !resolved[finding.FindingID] {
				remaining = true
			}
		}
		status := domain.StatusReview
		if remaining {
			status = domain.StatusRemediation
		}
		summary := struct {
			RoundNo            int      `json:"roundNo"`
			ProtocolRevisionID string   `json:"protocolRevisionId"`
			ObservationIDs     []string `json:"observationIds"`
		}{RoundNo: cmd.Control.RoundNo, ProtocolRevisionID: protocol.RevisionID}
		for _, o := range observations {
			summary.ObservationIDs = append(summary.ObservationIDs, o.ObservationID)
		}
		if err := tx.AppendAudit(ctx, caseID, "observation.batch_submitted", observations[0].OperatorID, "原子提交同轮对照区与关联试验区观察并完成逐区评估", summary); err != nil {
			return "", "", err
		}
		return status, "", nil
	})
}

func observationFromInput(caseID, baselineID string, input ObservationInput, submittedAt time.Time) domain.TrialObservation {
	return domain.TrialObservation{ObservationID: newID("observation"), CaseID: caseID, ZoneID: input.ZoneID, ProtocolRevisionID: input.ProtocolRevisionID, BaselineRevisionID: baselineID, RoundNo: input.RoundNo, ObservedAt: input.ObservedAt, ColorDeltaE: input.ColorDeltaE, ParticleLossScore: input.ParticleLossScore, MoisturePercent: input.MoisturePercent, ResidueValue: input.ResidueValue, EvidenceDigest: strings.TrimSpace(input.EvidenceDigest), EvidenceSummary: strings.TrimSpace(input.EvidenceSummary), OperatorID: strings.TrimSpace(input.OperatorID), SubmittedAt: submittedAt}
}

func snapshotHasRule(snapshot domain.EvaluationSnapshot, revisionID, ruleCode string) bool {
	for _, finding := range snapshot.Findings {
		if finding.ProtocolRevisionID == revisionID && finding.RuleCode == ruleCode && finding.Status == domain.FindingOpen {
			return true
		}
	}
	return false
}
