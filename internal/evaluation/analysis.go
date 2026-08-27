package evaluation

import (
	"math"
	"sort"

	"mural-conservation-gate/internal/domain"
)

type metricDefinition struct {
	code      string
	label     string
	value     func(domain.TrialObservation) float64
	threshold func(domain.SafetyThresholds) float64
}

var metricDefinitions = []metricDefinition{
	{code: "colorDeltaE", label: "前后色差", value: func(o domain.TrialObservation) float64 { return o.ColorDeltaE }, threshold: func(t domain.SafetyThresholds) float64 { return t.MaxColorDeltaE }},
	{code: "particleLossScore", label: "表面颗粒脱落", value: func(o domain.TrialObservation) float64 { return o.ParticleLossScore }, threshold: func(t domain.SafetyThresholds) float64 { return t.MaxParticleLossScore }},
	{code: "moisturePercent", label: "含水率", value: func(o domain.TrialObservation) float64 { return o.MoisturePercent }, threshold: func(t domain.SafetyThresholds) float64 { return t.MaxMoisturePercent }},
	{code: "residueValue", label: "残留指标", value: func(o domain.TrialObservation) float64 { return o.ResidueValue }, threshold: func(t domain.SafetyThresholds) float64 { return t.MaxResidueValue }},
}

func buildMetricAssessments(input Input) []domain.MetricAssessment {
	result := make([]domain.MetricAssessment, 0, len(metricDefinitions))
	control, hasControl := matchingControl(input)
	for _, definition := range metricDefinitions {
		observed := definition.value(input.Current)
		threshold := definition.threshold(input.Protocol.SafetyThresholds)
		assessment := domain.MetricAssessment{Code: definition.code, Label: definition.label, ObservedValue: round(observed), ThresholdValue: round(threshold), ThresholdMargin: round(threshold - observed), Passed: observed <= threshold}
		if hasControl {
			controlValue := round(definition.value(control))
			difference := round(observed - controlValue)
			assessment.ControlValue = &controlValue
			assessment.ControlDifference = &difference
			if difference > input.Protocol.SafetyThresholds.MaxControlDifference {
				assessment.Passed = false
			}
		}
		if previous, ok := previousObservation(input.History, input.Current); ok {
			previousValue := round(definition.value(previous))
			assessment.PreviousValue = &previousValue
			trend := percentChange(previousValue, observed)
			assessment.TrendPercent = &trend
		}
		result = append(result, assessment)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Code < result[j].Code })
	return result
}

func previousObservation(history []domain.TrialObservation, current domain.TrialObservation) (domain.TrialObservation, bool) {
	var selected domain.TrialObservation
	found := false
	for _, item := range history {
		if item.ZoneID != current.ZoneID || item.ProtocolRevisionID != current.ProtocolRevisionID || item.BaselineRevisionID != current.BaselineRevisionID || item.RoundNo >= current.RoundNo {
			continue
		}
		if !found || item.RoundNo > selected.RoundNo {
			selected, found = item, true
		}
	}
	return selected, found
}

func percentChange(previous, current float64) float64 {
	if math.Abs(previous) < 1e-9 {
		if math.Abs(current) < 1e-9 {
			return 0
		}
		return 100
	}
	return round((current - previous) / math.Abs(previous) * 100)
}

func round(value float64) float64 { return math.Round(value*1000) / 1000 }
