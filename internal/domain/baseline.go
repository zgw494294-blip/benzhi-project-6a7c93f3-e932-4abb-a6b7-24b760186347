package domain

import (
	"fmt"
	"math"
	"strings"
)

func SameBaselineMeasurements(a, b BaselineRevision) bool {
	return a.ColorL == b.ColorL && a.ColorA == b.ColorA && a.ColorB == b.ColorB && a.MoisturePercent == b.MoisturePercent &&
		a.ParticleCondition == b.ParticleCondition && a.ResidueBaseline == b.ResidueBaseline &&
		a.Ambient.TemperatureC == b.Ambient.TemperatureC && a.Ambient.HumidityPercent == b.Ambient.HumidityPercent && a.Ambient.LightLux == b.Ambient.LightLux
}

func CalculateBaselineImpact(previous, current BaselineRevision) BaselineImpact {
	dl, da, db := current.ColorL-previous.ColorL, current.ColorA-previous.ColorA, current.ColorB-previous.ColorB
	delta := BaselineDelta{ColorDeltaE: math.Sqrt(dl*dl + da*da + db*db), MoisturePercent: current.MoisturePercent - previous.MoisturePercent, ResidueBaseline: current.ResidueBaseline - previous.ResidueBaseline, TemperatureC: current.Ambient.TemperatureC - previous.Ambient.TemperatureC, HumidityPercent: current.Ambient.HumidityPercent - previous.Ambient.HumidityPercent, LightLux: current.Ambient.LightLux - previous.Ambient.LightLux}
	return BaselineImpact{PreviousRevisionID: previous.RevisionID, CurrentRevisionID: current.RevisionID, Delta: delta, Summary: fmt.Sprintf("色度差 %.3f，含水率变化 %+.3f，残留变化 %+.3f，温度变化 %+.2f℃，湿度变化 %+.2f%%，照度变化 %+.2f lux", delta.ColorDeltaE, delta.MoisturePercent, delta.ResidueBaseline, delta.TemperatureC, delta.HumidityPercent, delta.LightLux)}
}

func ValidateBaselineRetest(previous, current BaselineRevision) error {
	if strings.TrimSpace(current.Reason) == "" {
		return Invalid("reason", "基线复测必须填写原因")
	}
	if !current.MeasuredAt.After(previous.MeasuredAt) {
		return Invalid("measuredAt", "复测时间必须晚于当前基线测量时间")
	}
	if SameBaselineMeasurements(previous, current) {
		return Invalid("baseline", "复测内容与当前基线完全相同")
	}
	return nil
}

func MissingEvidence(readiness WorkflowReadiness) []string {
	result := []string{}
	for _, check := range readiness.Checks {
		if !check.Passed {
			result = append(result, check.Detail)
		}
	}
	return result
}
