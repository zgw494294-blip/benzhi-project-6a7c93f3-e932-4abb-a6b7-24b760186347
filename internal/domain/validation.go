package domain

import (
	"math"
	"strings"
	"time"
)

func ValidateNewCase(c ConservationCase) error {
	if strings.TrimSpace(c.SiteName) == "" {
		return Invalid("siteName", "必须填写建筑或遗址名称")
	}
	if strings.TrimSpace(c.MuralLocation) == "" {
		return Invalid("muralLocation", "必须填写壁画空间位置")
	}
	if len(c.MaterialLayers) == 0 {
		return Invalid("materialLayers", "至少登记一个材质层")
	}
	for _, layer := range c.MaterialLayers {
		if strings.TrimSpace(layer) == "" {
			return Invalid("materialLayers", "材质层不得为空")
		}
	}
	if len(c.Pathologies) == 0 {
		return Invalid("pathologies", "至少登记一种病害")
	}
	return ValidateAmbient(c.AmbientCondition)
}

func ValidateAmbient(a AmbientCondition) error {
	if !finite(a.TemperatureC) || a.TemperatureC < -20 || a.TemperatureC > 60 {
		return Invalid("ambientCondition.temperatureC", "温度必须在 -20 到 60 摄氏度之间")
	}
	if !finite(a.HumidityPercent) || a.HumidityPercent < 0 || a.HumidityPercent > 100 {
		return Invalid("ambientCondition.humidityPercent", "相对湿度必须在 0 到 100 之间")
	}
	if !finite(a.LightLux) || a.LightLux < 0 {
		return Invalid("ambientCondition.lightLux", "照度不得为负数")
	}
	return nil
}

func ValidateBaseline(b BaselineRevision) error {
	if b.RevisionNo < 1 {
		return Invalid("revisionNo", "基线版本必须从 1 开始")
	}
	if b.ColorL < 0 || b.ColorL > 100 || !finite(b.ColorL) {
		return Invalid("colorL", "L 值必须在 0 到 100 之间")
	}
	if b.ColorA < -128 || b.ColorA > 127 || !finite(b.ColorA) || b.ColorB < -128 || b.ColorB > 127 || !finite(b.ColorB) {
		return Invalid("color", "a、b 色度值超出有效范围")
	}
	if b.MoisturePercent < 0 || b.MoisturePercent > 100 || !finite(b.MoisturePercent) {
		return Invalid("moisturePercent", "含水率必须在 0 到 100 之间")
	}
	if b.ResidueBaseline < 0 || !finite(b.ResidueBaseline) {
		return Invalid("residueBaseline", "残留基线不得为负数")
	}
	if strings.TrimSpace(b.ParticleCondition) == "" {
		return Invalid("particleCondition", "必须描述表面颗粒状态")
	}
	if strings.TrimSpace(b.MeasuredBy) == "" {
		return Invalid("measuredBy", "必须登记测量人员")
	}
	if b.MeasuredAt.IsZero() || b.MeasuredAt.After(time.Now().Add(5*time.Minute)) {
		return Invalid("measuredAt", "测量时间无效")
	}
	return ValidateAmbient(b.Ambient)
}

func ValidateZone(z TrialZone) error {
	if z.ZoneType != ZoneTrial && z.ZoneType != ZoneControl {
		return Invalid("zoneType", "分区类型只能为 trial 或 control")
	}
	if len(z.BoundaryPoints) < 3 {
		return Invalid("boundaryPoints", "边界至少需要三个坐标点")
	}
	for _, p := range z.BoundaryPoints {
		if !finite(p.X) || !finite(p.Y) {
			return Invalid("boundaryPoints", "边界坐标必须是有限数值")
		}
	}
	computed := PolygonArea(z.BoundaryPoints)
	if computed <= 0 {
		return Invalid("boundaryPoints", "边界不能退化或自相消面积为零")
	}
	if z.AreaCM2 <= 0 || math.Abs(computed-z.AreaCM2)/computed > 0.15 {
		return Invalid("areaCm2", "登记面积与坐标计算面积偏差不得超过 15%")
	}
	if strings.TrimSpace(z.RepresentativeReason) == "" {
		return Invalid("representativeReason", "必须说明分区代表性")
	}
	if z.ZoneType == ZoneTrial && strings.TrimSpace(z.ControlZoneID) == "" {
		return Invalid("controlZoneId", "试验区必须关联对照区")
	}
	if z.ZoneType == ZoneControl && z.ControlZoneID != "" {
		return Invalid("controlZoneId", "对照区不得再关联对照区")
	}
	return nil
}

func ValidateProtocol(p CleaningProtocolRevision) error {
	if strings.TrimSpace(p.ProtocolID) == "" {
		return Invalid("protocolId", "必须提供候选方案标识")
	}
	if p.RevisionNo < 1 {
		return Invalid("revisionNo", "方案修订号必须从 1 开始")
	}
	if len(p.Ingredients) == 0 {
		return Invalid("ingredients", "至少需要一种材料成分")
	}
	total := 0.0
	for _, item := range p.Ingredients {
		if strings.TrimSpace(item.Name) == "" || item.Percentage <= 0 || !finite(item.Percentage) {
			return Invalid("ingredients", "材料名称和占比必须有效")
		}
		total += item.Percentage
	}
	if total > 100.001 {
		return Invalid("ingredients", "成分占比合计不得超过 100")
	}
	if p.Concentration <= 0 || p.Concentration > 100 || !finite(p.Concentration) {
		return Invalid("concentration", "浓度必须大于 0 且不超过 100")
	}
	if p.ContactSeconds < 1 || p.ContactSeconds > 86400 {
		return Invalid("contactSeconds", "接触时长必须在 1 秒到 24 小时之间")
	}
	if len(p.Tools) == 0 || len(p.RemovalSteps) == 0 {
		return Invalid("tools", "必须登记工具和清除步骤")
	}
	if strings.TrimSpace(p.CreatedBy) == "" {
		return Invalid("createdBy", "必须登记方案编制人")
	}
	return ValidateThresholds(p.SafetyThresholds)
}

func ValidateThresholds(t SafetyThresholds) error {
	values := []float64{t.MaxColorDeltaE, t.MaxParticleLossScore, t.MaxMoisturePercent, t.MaxResidueValue, t.MaxControlDifference}
	for _, value := range values {
		if value <= 0 || !finite(value) {
			return Invalid("safetyThresholds", "所有安全阈值必须为有限正数")
		}
	}
	if t.MaxParticleLossScore > 10 || t.MaxMoisturePercent > 100 {
		return Invalid("safetyThresholds", "颗粒脱落和含水率阈值超出量程")
	}
	return nil
}

func ValidateObservation(o TrialObservation) error {
	if o.RoundNo < 1 {
		return Invalid("roundNo", "试验轮次必须从 1 开始")
	}
	if strings.TrimSpace(o.ZoneID) == "" || strings.TrimSpace(o.ProtocolRevisionID) == "" {
		return Invalid("zoneId", "必须关联分区和方案修订")
	}
	if o.ColorDeltaE < 0 || !finite(o.ColorDeltaE) || o.ParticleLossScore < 0 || o.ParticleLossScore > 10 || !finite(o.ParticleLossScore) {
		return Invalid("observation", "色差或颗粒脱落评分无效")
	}
	if o.MoisturePercent < 0 || o.MoisturePercent > 100 || !finite(o.MoisturePercent) || o.ResidueValue < 0 || !finite(o.ResidueValue) {
		return Invalid("observation", "含水率或残留指标无效")
	}
	if o.ObservedAt.IsZero() || o.ObservedAt.After(time.Now().Add(5*time.Minute)) {
		return Invalid("observedAt", "观察时间无效")
	}
	if strings.TrimSpace(o.EvidenceDigest) == "" || strings.TrimSpace(o.EvidenceSummary) == "" {
		return Invalid("evidence", "必须提供证据摘要与校验值")
	}
	if strings.TrimSpace(o.OperatorID) == "" {
		return Invalid("operatorId", "必须登记试验人员")
	}
	return nil
}

func PolygonArea(points []Point) float64 {
	if len(points) < 3 {
		return 0
	}
	sum := 0.0
	for i, p := range points {
		next := points[(i+1)%len(points)]
		sum += p.X*next.Y - next.X*p.Y
	}
	return math.Abs(sum) / 2
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
