package evaluation

import "mural-conservation-gate/internal/domain"

type Rule struct {
	Code      string
	Severity  domain.Severity
	Message   string
	Value     func(domain.TrialObservation) float64
	Threshold func(domain.SafetyThresholds) float64
}

var thresholdRules = []Rule{
	{Code: "COLOR_DELTA_LIMIT", Severity: domain.SeverityBlocking, Message: "色差超过方案安全阈值", Value: func(o domain.TrialObservation) float64 { return o.ColorDeltaE }, Threshold: func(t domain.SafetyThresholds) float64 { return t.MaxColorDeltaE }},
	{Code: "PARTICLE_LOSS_LIMIT", Severity: domain.SeverityBlocking, Message: "表面颗粒脱落超过方案安全阈值", Value: func(o domain.TrialObservation) float64 { return o.ParticleLossScore }, Threshold: func(t domain.SafetyThresholds) float64 { return t.MaxParticleLossScore }},
	{Code: "MOISTURE_LIMIT", Severity: domain.SeverityBlocking, Message: "含水率超过方案安全阈值", Value: func(o domain.TrialObservation) float64 { return o.MoisturePercent }, Threshold: func(t domain.SafetyThresholds) float64 { return t.MaxMoisturePercent }},
	{Code: "RESIDUE_LIMIT", Severity: domain.SeverityBlocking, Message: "材料残留超过方案安全阈值", Value: func(o domain.TrialObservation) float64 { return o.ResidueValue }, Threshold: func(t domain.SafetyThresholds) float64 { return t.MaxResidueValue }},
}
