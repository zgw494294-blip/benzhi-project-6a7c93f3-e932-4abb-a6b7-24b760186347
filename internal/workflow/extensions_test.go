package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"mural-conservation-gate/internal/domain"
	"mural-conservation-gate/internal/store"
)

func TestQueueCursorAndBaselineRetestIsolation(t *testing.T) {
	repository, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := New(repository)
	ctx := context.Background()
	for _, name := range []string{"永宁寺甲区", "永宁寺乙区", "石窟丙区"} {
		if _, err = service.CreateCase(ctx, CreateCaseCommand{SiteName: name, MuralLocation: "东壁", MaterialLayers: []string{"颜料层"}, Pathologies: []string{"粉化"}, AmbientCondition: domain.AmbientCondition{TemperatureC: 20, HumidityPercent: 50, LightLux: 80}, ActorID: "tester"}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := service.ListCases(ctx, ListCasesQuery{Keyword: "永宁寺", KeywordProvided: true, Limit: 1})
	if err != nil || len(first.Items) != 1 || first.NextCursor == "" || first.Total != 2 {
		t.Fatalf("第一页不完整: %#v, %v", first, err)
	}
	second, err := service.ListCases(ctx, ListCasesQuery{Keyword: "永宁寺", KeywordProvided: true, Cursor: first.NextCursor, Limit: 1})
	if err != nil || len(second.Items) != 1 || second.Items[0].Case.CaseID == first.Items[0].Case.CaseID {
		t.Fatalf("稳定游标分页出现重复: %#v, %v", second, err)
	}
	if _, err = service.ListCases(ctx, ListCasesQuery{Status: "unknown", Limit: 10}); !domain.IsValidation(err) {
		t.Fatalf("未知状态应返回字段错误: %v", err)
	}

	view, controlID, trialID, revisionID := prepareEvaluableCase(t, service, ctx)
	observedAt := time.Now().UTC()
	view, err = service.SubmitPairedObservations(ctx, view.Case.CaseID, PairedObservationBatchCommand{ExpectedVersion: view.Case.Version, Control: passingInput(controlID, revisionID, 1, observedAt, "control-1"), Trials: []ObservationInput{passingInput(trialID, revisionID, 1, observedAt, "trial-1")}})
	if err != nil || view.Case.Status != domain.StatusReview || len(view.Snapshots) != 1 {
		t.Fatalf("首次成对观察未完成评估: %v, %#v", err, view)
	}
	retestAt := time.Now().UTC().Add(time.Second)
	view, err = service.SubmitBaseline(ctx, view.Case.CaseID, BaselineCommand{ExpectedVersion: view.Case.Version, ColorL: 53, ColorA: 8, ColorB: 12, MoisturePercent: 4.3, ParticleCondition: "复测后局部稳定", ResidueBaseline: .3, MeasuredAt: retestAt, MeasuredBy: "tester", Ambient: domain.AmbientCondition{TemperatureC: 21, HumidityPercent: 52, LightLux: 85}, Reason: "环境湿度变化后复测"})
	if err != nil || view.Case.Status != domain.StatusTesting || len(view.Baselines) != 2 || view.Baselines[1].Impact == nil || !view.Snapshots[0].Historical {
		t.Fatalf("基线复测历史隔离失败: %v, %#v", err, view)
	}
	comparison, err := service.CandidateComparisons(ctx, view.Case.CaseID)
	if err != nil || len(comparison.Candidates) != 1 || comparison.Candidates[0].Eligible {
		t.Fatalf("未补当前基线证据时不应可入围: %v, %#v", err, comparison)
	}
	view, err = service.SubmitPairedObservations(ctx, view.Case.CaseID, PairedObservationBatchCommand{ExpectedVersion: view.Case.Version, Control: passingInput(controlID, revisionID, 2, retestAt.Add(time.Second), "control-2"), Trials: []ObservationInput{passingInput(trialID, revisionID, 2, retestAt.Add(time.Second), "trial-2")}})
	if err != nil {
		t.Fatal(err)
	}
	comparison, err = service.CandidateComparisons(ctx, view.Case.CaseID)
	if err != nil || !comparison.Candidates[0].Eligible {
		t.Fatalf("补齐当前基线成对证据后应可入围: %v, %#v", err, comparison)
	}
}

func TestPairedBatchRejectsWholePayload(t *testing.T) {
	repository, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := New(repository)
	ctx := context.Background()
	view, controlID, trialID, revisionID := prepareEvaluableCase(t, service, ctx)
	invalid := passingInput(trialID, revisionID, 1, time.Now().UTC(), "trial")
	invalid.ZoneID = controlID
	_, err = service.SubmitPairedObservations(ctx, view.Case.CaseID, PairedObservationBatchCommand{ExpectedVersion: view.Case.Version, Control: passingInput(controlID, revisionID, 1, time.Now().UTC(), "control"), Trials: []ObservationInput{invalid}})
	if !domain.IsValidation(err) {
		t.Fatalf("关联错误应拒绝整批: %v", err)
	}
	stored, loadErr := service.GetCase(ctx, view.Case.CaseID)
	if loadErr != nil || len(stored.Observations) != 0 || len(stored.Snapshots) != 0 || stored.Case.Version != view.Case.Version {
		t.Fatalf("失败批次产生了部分记录: %v, %#v", loadErr, stored)
	}
	if errors.Is(err, domain.ErrConflict) {
		t.Fatalf("关联错误应是字段错误而不是版本冲突: %v", err)
	}
}

func TestRemediationClosesOnlyAfterPlannedPairedRetest(t *testing.T) {
	repository, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := New(repository)
	ctx := context.Background()
	view, controlID, trialID, revisionID := prepareEvaluableCase(t, service, ctx)
	control := passingInput(controlID, revisionID, 1, time.Now().UTC(), "control-risk")
	control.ColorDeltaE = .2
	trial := passingInput(trialID, revisionID, 1, control.ObservedAt, "trial-risk")
	trial.ColorDeltaE = 5
	view, err = service.SubmitPairedObservations(ctx, view.Case.CaseID, PairedObservationBatchCommand{ExpectedVersion: view.Case.Version, Control: control, Trials: []ObservationInput{trial}})
	if err != nil || view.Case.Status != domain.StatusRemediation || len(view.Findings) == 0 {
		t.Fatalf("超阈值批次应生成整改风险: %v, %#v", err, view)
	}
	concentration, contact := 4.0, 20
	view, err = service.CreateRemediation(ctx, view.Case.CaseID, RemediationCommand{ExpectedVersion: view.Case.Version, FindingID: view.Findings[0].FindingID, Assignee: "engineer", Analysis: "浓度和接触时长偏高", PlannedParameters: domain.ParameterPlan{Concentration: &concentration, ContactSeconds: &contact}, RequiredZoneIDs: []string{trialID}, DueAt: time.Now().UTC().Add(24 * time.Hour), ActorID: "engineer"})
	if err != nil || len(view.Remediations) != 1 || view.Remediations[0].Status != domain.RemediationPending {
		t.Fatalf("整改任务创建失败: %v, %#v", err, view.Remediations)
	}
	view, err = service.ReviseProtocol(ctx, view.Case.CaseID, ProtocolCommand{ExpectedVersion: view.Case.Version, ProtocolID: "candidate-a", Ingredients: []domain.Ingredient{{Name: "凝胶", Percentage: 95}}, Concentration: concentration, ContactSeconds: contact, Tools: []string{"棉签"}, RemovalSteps: []string{"点涂", "吸除"}, SafetyThresholds: domain.SafetyThresholds{MaxColorDeltaE: 3, MaxParticleLossScore: 1.5, MaxMoisturePercent: 10, MaxResidueValue: 2, MaxControlDifference: 2}, ChangeReason: "按色差整改计划降低参数", CreatedBy: "engineer"})
	if err != nil || view.Remediations[0].Status != domain.RemediationRetest {
		t.Fatalf("按计划修订后应进入待复验: %v, %#v", err, view.Remediations)
	}
	newRevisionID := view.Protocols[len(view.Protocols)-1].RevisionID
	control = passingInput(controlID, newRevisionID, 1, time.Now().UTC(), "control-retest")
	control.ColorDeltaE = .2
	trial = passingInput(trialID, newRevisionID, 1, control.ObservedAt, "trial-retest")
	view, err = service.SubmitPairedObservations(ctx, view.Case.CaseID, PairedObservationBatchCommand{ExpectedVersion: view.Case.Version, Control: control, Trials: []ObservationInput{trial}})
	if err != nil || view.Remediations[0].Status != domain.RemediationResolved || view.Findings[0].Status != domain.FindingClosed || len(view.Remediations[0].ResolvedSnapshotIDs) != 1 {
		t.Fatalf("目标规则通过后未完成证据销项: %v, %#v, %#v", err, view.Remediations, view.Findings)
	}
}

func prepareEvaluableCase(t *testing.T, service *Service, ctx context.Context) (domain.CaseView, string, string, string) {
	t.Helper()
	c, err := service.CreateCase(ctx, CreateCaseCommand{SiteName: "测试遗址", MuralLocation: "东壁", MaterialLayers: []string{"颜料层"}, Pathologies: []string{"粉化"}, AmbientCondition: domain.AmbientCondition{TemperatureC: 20, HumidityPercent: 50, LightLux: 80}, ActorID: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	view, err := service.SubmitBaseline(ctx, c.CaseID, BaselineCommand{ExpectedVersion: c.Version, ColorL: 52, ColorA: 8, ColorB: 12, MoisturePercent: 4, ParticleCondition: "局部稳定", ResidueBaseline: .2, MeasuredAt: time.Now().UTC().Add(-time.Hour), MeasuredBy: "tester", Ambient: c.AmbientCondition})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.AddZone(ctx, c.CaseID, ZoneCommand{ExpectedVersion: view.Case.Version, ZoneType: domain.ZoneControl, BoundaryPoints: []domain.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}}, AreaCM2: 100, RepresentativeReason: "典型对照", ActorID: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	controlID := view.Zones[0].ZoneID
	view, err = service.AddZone(ctx, c.CaseID, ZoneCommand{ExpectedVersion: view.Case.Version, ZoneType: domain.ZoneTrial, BoundaryPoints: []domain.Point{{X: 20, Y: 0}, {X: 30, Y: 0}, {X: 30, Y: 10}, {X: 20, Y: 10}}, AreaCM2: 100, RepresentativeReason: "典型试验", ControlZoneID: controlID, ActorID: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	trialID := view.Zones[1].ZoneID
	view, err = service.ReviseProtocol(ctx, c.CaseID, ProtocolCommand{ExpectedVersion: view.Case.Version, ProtocolID: "candidate-a", Ingredients: []domain.Ingredient{{Name: "凝胶", Percentage: 95}}, Concentration: 5, ContactSeconds: 30, Tools: []string{"棉签"}, RemovalSteps: []string{"点涂", "吸除"}, SafetyThresholds: domain.SafetyThresholds{MaxColorDeltaE: 3, MaxParticleLossScore: 1.5, MaxMoisturePercent: 10, MaxResidueValue: 2, MaxControlDifference: 2}, ChangeReason: "首版", CreatedBy: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	return view, controlID, trialID, view.Protocols[0].RevisionID
}

func passingInput(zoneID, revisionID string, round int, observedAt time.Time, digest string) ObservationInput {
	return ObservationInput{ZoneID: zoneID, ProtocolRevisionID: revisionID, RoundNo: round, ObservedAt: observedAt, ColorDeltaE: 1, ParticleLossScore: .2, MoisturePercent: 4.2, ResidueValue: .4, EvidenceDigest: digest, EvidenceSummary: "现场测量与影像证据完整", OperatorID: "tester"}
}
