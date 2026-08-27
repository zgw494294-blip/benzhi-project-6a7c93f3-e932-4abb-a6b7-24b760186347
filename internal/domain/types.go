package domain

import "time"

type CaseStatus string

const (
	StatusDraft       CaseStatus = "draft"
	StatusTesting     CaseStatus = "testing"
	StatusRemediation CaseStatus = "remediation"
	StatusReview      CaseStatus = "review"
	StatusFrozen      CaseStatus = "frozen"
	StatusPermitted   CaseStatus = "permitted"
)

type ZoneType string

const (
	ZoneTrial   ZoneType = "trial"
	ZoneControl ZoneType = "control"
)

type FindingStatus string

const (
	FindingOpen   FindingStatus = "open"
	FindingClosed FindingStatus = "closed"
)

type Severity string

const (
	SeverityWarning  Severity = "warning"
	SeverityBlocking Severity = "blocking"
)

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type AmbientCondition struct {
	TemperatureC    float64 `json:"temperatureC"`
	HumidityPercent float64 `json:"humidityPercent"`
	LightLux        float64 `json:"lightLux"`
}

type BaselineRevision struct {
	RevisionID        string           `json:"revisionId"`
	RevisionNo        int              `json:"revisionNo"`
	ColorL            float64          `json:"colorL"`
	ColorA            float64          `json:"colorA"`
	ColorB            float64          `json:"colorB"`
	MoisturePercent   float64          `json:"moisturePercent"`
	ParticleCondition string           `json:"particleCondition"`
	ResidueBaseline   float64          `json:"residueBaseline"`
	MeasuredAt        time.Time        `json:"measuredAt"`
	MeasuredBy        string           `json:"measuredBy"`
	Ambient           AmbientCondition `json:"ambient"`
	CreatedAt         time.Time        `json:"createdAt"`
	Reason            string           `json:"reason,omitempty"`
	Impact            *BaselineImpact  `json:"impact,omitempty"`
}

type ConservationCase struct {
	CaseID             string           `json:"caseId"`
	SiteName           string           `json:"siteName"`
	MuralLocation      string           `json:"muralLocation"`
	MaterialLayers     []string         `json:"materialLayers"`
	Pathologies        []string         `json:"pathologies"`
	AmbientCondition   AmbientCondition `json:"ambientCondition"`
	BaselineRevisionID string           `json:"baselineRevisionId,omitempty"`
	Status             CaseStatus       `json:"status"`
	Version            int64            `json:"version"`
	CreatedAt          time.Time        `json:"createdAt"`
	UpdatedAt          time.Time        `json:"updatedAt"`
}

type TrialZone struct {
	ZoneID               string    `json:"zoneId"`
	CaseID               string    `json:"caseId"`
	ZoneType             ZoneType  `json:"zoneType"`
	BoundaryPoints       []Point   `json:"boundaryPoints"`
	AreaCM2              float64   `json:"areaCm2"`
	RepresentativeReason string    `json:"representativeReason"`
	ControlZoneID        string    `json:"controlZoneId,omitempty"`
	CreatedAt            time.Time `json:"createdAt"`
}

type Ingredient struct {
	Name       string  `json:"name"`
	Percentage float64 `json:"percentage"`
}

type SafetyThresholds struct {
	MaxColorDeltaE       float64 `json:"maxColorDeltaE"`
	MaxParticleLossScore float64 `json:"maxParticleLossScore"`
	MaxMoisturePercent   float64 `json:"maxMoisturePercent"`
	MaxResidueValue      float64 `json:"maxResidueValue"`
	MaxControlDifference float64 `json:"maxControlDifference"`
}

type CleaningProtocolRevision struct {
	RevisionID       string           `json:"revisionId"`
	CaseID           string           `json:"caseId"`
	ProtocolID       string           `json:"protocolId"`
	RevisionNo       int              `json:"revisionNo"`
	Ingredients      []Ingredient     `json:"ingredients"`
	Concentration    float64          `json:"concentration"`
	ContactSeconds   int              `json:"contactSeconds"`
	Tools            []string         `json:"tools"`
	RemovalSteps     []string         `json:"removalSteps"`
	SafetyThresholds SafetyThresholds `json:"safetyThresholds"`
	ChangeReason     string           `json:"changeReason"`
	CreatedBy        string           `json:"createdBy"`
	CreatedAt        time.Time        `json:"createdAt"`
}

type TrialObservation struct {
	ObservationID      string    `json:"observationId"`
	CaseID             string    `json:"caseId"`
	ZoneID             string    `json:"zoneId"`
	ProtocolRevisionID string    `json:"protocolRevisionId"`
	BaselineRevisionID string    `json:"baselineRevisionId"`
	RoundNo            int       `json:"roundNo"`
	ObservedAt         time.Time `json:"observedAt"`
	ColorDeltaE        float64   `json:"colorDeltaE"`
	ParticleLossScore  float64   `json:"particleLossScore"`
	MoisturePercent    float64   `json:"moisturePercent"`
	ResidueValue       float64   `json:"residueValue"`
	EvidenceDigest     string    `json:"evidenceDigest"`
	EvidenceSummary    string    `json:"evidenceSummary"`
	OperatorID         string    `json:"operatorId"`
	SubmittedAt        time.Time `json:"submittedAt"`
}

type RiskFinding struct {
	FindingID               string        `json:"findingId"`
	CaseID                  string        `json:"caseId"`
	ProtocolRevisionID      string        `json:"protocolRevisionId"`
	BaselineRevisionID      string        `json:"baselineRevisionId"`
	RuleCode                string        `json:"ruleCode"`
	Severity                Severity      `json:"severity"`
	Message                 string        `json:"message"`
	EvidenceRefs            []string      `json:"evidenceRefs"`
	Status                  FindingStatus `json:"status"`
	RemediationNote         string        `json:"remediationNote,omitempty"`
	ResolvedByObservationID string        `json:"resolvedByObservationId,omitempty"`
	OpenedAt                time.Time     `json:"openedAt"`
	ClosedAt                *time.Time    `json:"closedAt,omitempty"`
	Historical              bool          `json:"historical"`
}

type EvaluationSnapshot struct {
	SnapshotID         string             `json:"snapshotId"`
	CaseID             string             `json:"caseId"`
	ProtocolRevisionID string             `json:"protocolRevisionId"`
	BaselineRevisionID string             `json:"baselineRevisionId"`
	ObservationID      string             `json:"observationId"`
	Findings           []RiskFinding      `json:"findings"`
	EligibleForReview  bool               `json:"eligibleForReview"`
	Metrics            []MetricAssessment `json:"metrics"`
	EvaluatedAt        time.Time          `json:"evaluatedAt"`
	Historical         bool               `json:"historical"`
}

type MetricAssessment struct {
	Code              string   `json:"code"`
	Label             string   `json:"label"`
	ObservedValue     float64  `json:"observedValue"`
	ThresholdValue    float64  `json:"thresholdValue"`
	ThresholdMargin   float64  `json:"thresholdMargin"`
	ControlValue      *float64 `json:"controlValue,omitempty"`
	ControlDifference *float64 `json:"controlDifference,omitempty"`
	PreviousValue     *float64 `json:"previousValue,omitempty"`
	TrendPercent      *float64 `json:"trendPercent,omitempty"`
	Passed            bool     `json:"passed"`
}

type FrozenManifest struct {
	ManifestID         string                    `json:"manifestId"`
	CaseID             string                    `json:"caseId"`
	ProtocolRevisionID string                    `json:"protocolRevisionId"`
	ProtocolDigest     string                    `json:"protocolDigest"`
	ObservationIDs     []string                  `json:"observationIds"`
	EvidenceDigests    []string                  `json:"evidenceDigests"`
	EvidenceReferences []FrozenEvidenceReference `json:"evidenceReferences"`
	Digest             string                    `json:"digest"`
	ReviewerID         string                    `json:"reviewerId"`
	ReviewNote         string                    `json:"reviewNote"`
	ComparisonDigest   string                    `json:"comparisonDigest"`
	SelectionReason    string                    `json:"selectionReason"`
	FrozenAt           time.Time                 `json:"frozenAt"`
}

type FrozenEvidenceReference struct {
	ObservationID  string   `json:"observationId"`
	EvidenceDigest string   `json:"evidenceDigest"`
	ZoneID         string   `json:"zoneId"`
	ZoneType       ZoneType `json:"zoneType"`
	RoundNo        int      `json:"roundNo"`
}

type WorkPermit struct {
	PermitID                 string    `json:"permitId"`
	PermitNumber             string    `json:"permitNumber"`
	CaseID                   string    `json:"caseId"`
	FrozenProtocolRevisionID string    `json:"frozenProtocolRevisionId"`
	EvidenceManifestDigest   string    `json:"evidenceManifestDigest"`
	Scope                    string    `json:"scope"`
	Restrictions             []string  `json:"restrictions"`
	IssuedBy                 string    `json:"issuedBy"`
	IssuedAt                 time.Time `json:"issuedAt"`
	ExpiresAt                time.Time `json:"expiresAt"`
	VerificationDigest       string    `json:"verificationDigest"`
}

type AuditEvent struct {
	Sequence  int64     `json:"sequence"`
	CaseID    string    `json:"caseId"`
	EventType string    `json:"eventType"`
	ActorID   string    `json:"actorId"`
	Summary   string    `json:"summary"`
	Payload   jsonRaw   `json:"payload,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type jsonRaw = []byte

type CaseView struct {
	Case         ConservationCase           `json:"case"`
	Baseline     *BaselineRevision          `json:"baseline,omitempty"`
	Baselines    []BaselineRevision         `json:"baselines"`
	Zones        []TrialZone                `json:"zones"`
	Protocols    []CleaningProtocolRevision `json:"protocols"`
	Observations []TrialObservation         `json:"observations"`
	Findings     []RiskFinding              `json:"findings"`
	Snapshots    []EvaluationSnapshot       `json:"evaluationSnapshots"`
	Remediations []RemediationTask          `json:"remediations"`
	Selection    *CandidateSelection        `json:"candidateSelection,omitempty"`
	Manifest     *FrozenManifest            `json:"manifest,omitempty"`
	Permit       *WorkPermit                `json:"permit,omitempty"`
	Readiness    WorkflowReadiness          `json:"readiness"`
}
