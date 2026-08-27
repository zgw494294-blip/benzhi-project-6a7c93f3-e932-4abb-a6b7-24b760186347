package workflow

import (
	"time"

	"mural-conservation-gate/internal/domain"
)

type CreateCaseCommand struct {
	SiteName         string                  `json:"siteName"`
	MuralLocation    string                  `json:"muralLocation"`
	MaterialLayers   []string                `json:"materialLayers"`
	Pathologies      []string                `json:"pathologies"`
	AmbientCondition domain.AmbientCondition `json:"ambientCondition"`
	ActorID          string                  `json:"actorId"`
}

type BaselineCommand struct {
	ExpectedVersion   int64                   `json:"expectedVersion"`
	ColorL            float64                 `json:"colorL"`
	ColorA            float64                 `json:"colorA"`
	ColorB            float64                 `json:"colorB"`
	MoisturePercent   float64                 `json:"moisturePercent"`
	ParticleCondition string                  `json:"particleCondition"`
	ResidueBaseline   float64                 `json:"residueBaseline"`
	MeasuredAt        time.Time               `json:"measuredAt"`
	MeasuredBy        string                  `json:"measuredBy"`
	Ambient           domain.AmbientCondition `json:"ambient"`
	Reason            string                  `json:"reason,omitempty"`
}

type ZoneCommand struct {
	ExpectedVersion      int64           `json:"expectedVersion"`
	ZoneType             domain.ZoneType `json:"zoneType"`
	BoundaryPoints       []domain.Point  `json:"boundaryPoints"`
	AreaCM2              float64         `json:"areaCm2"`
	RepresentativeReason string          `json:"representativeReason"`
	ControlZoneID        string          `json:"controlZoneId"`
	ActorID              string          `json:"actorId"`
}

type ProtocolCommand struct {
	ExpectedVersion  int64                   `json:"expectedVersion"`
	ProtocolID       string                  `json:"protocolId"`
	Ingredients      []domain.Ingredient     `json:"ingredients"`
	Concentration    float64                 `json:"concentration"`
	ContactSeconds   int                     `json:"contactSeconds"`
	Tools            []string                `json:"tools"`
	RemovalSteps     []string                `json:"removalSteps"`
	SafetyThresholds domain.SafetyThresholds `json:"safetyThresholds"`
	ChangeReason     string                  `json:"changeReason"`
	CreatedBy        string                  `json:"createdBy"`
}

type ObservationCommand struct {
	ExpectedVersion    int64     `json:"expectedVersion"`
	ZoneID             string    `json:"zoneId"`
	ProtocolRevisionID string    `json:"protocolRevisionId"`
	RoundNo            int       `json:"roundNo"`
	ObservedAt         time.Time `json:"observedAt"`
	ColorDeltaE        float64   `json:"colorDeltaE"`
	ParticleLossScore  float64   `json:"particleLossScore"`
	MoisturePercent    float64   `json:"moisturePercent"`
	ResidueValue       float64   `json:"residueValue"`
	EvidenceDigest     string    `json:"evidenceDigest"`
	EvidenceSummary    string    `json:"evidenceSummary"`
	OperatorID         string    `json:"operatorId"`
}

type PairedObservationBatchCommand struct {
	ExpectedVersion int64              `json:"expectedVersion"`
	Control         ObservationInput   `json:"control"`
	Trials          []ObservationInput `json:"trials"`
}

type ObservationInput struct {
	ZoneID             string    `json:"zoneId"`
	ProtocolRevisionID string    `json:"protocolRevisionId"`
	RoundNo            int       `json:"roundNo"`
	ObservedAt         time.Time `json:"observedAt"`
	ColorDeltaE        float64   `json:"colorDeltaE"`
	ParticleLossScore  float64   `json:"particleLossScore"`
	MoisturePercent    float64   `json:"moisturePercent"`
	ResidueValue       float64   `json:"residueValue"`
	EvidenceDigest     string    `json:"evidenceDigest"`
	EvidenceSummary    string    `json:"evidenceSummary"`
	OperatorID         string    `json:"operatorId"`
}

type SelectionCommand struct {
	ExpectedVersion    int64  `json:"expectedVersion"`
	ProtocolRevisionID string `json:"protocolRevisionId"`
	Reason             string `json:"reason"`
	ReviewerID         string `json:"reviewerId"`
}

type RemediationCommand struct {
	ExpectedVersion   int64                `json:"expectedVersion"`
	FindingID         string               `json:"findingId"`
	Assignee          string               `json:"assignee"`
	Analysis          string               `json:"analysis"`
	PlannedParameters domain.ParameterPlan `json:"plannedParameters"`
	RequiredZoneIDs   []string             `json:"requiredZoneIds"`
	DueAt             time.Time            `json:"dueAt"`
	ActorID           string               `json:"actorId"`
}

type FreezeCommand struct {
	ExpectedVersion    int64  `json:"expectedVersion"`
	ProtocolRevisionID string `json:"protocolRevisionId"`
	ReviewerID         string `json:"reviewerId"`
	ReviewNote         string `json:"reviewNote"`
}

type PermitCommand struct {
	ExpectedVersion int64     `json:"expectedVersion"`
	Scope           string    `json:"scope"`
	Restrictions    []string  `json:"restrictions"`
	IssuedBy        string    `json:"issuedBy"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

type Verification = domain.PermitReceipt
