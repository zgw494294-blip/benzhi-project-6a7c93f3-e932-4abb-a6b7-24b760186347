package domain

import "time"

type BaselineDelta struct {
	ColorDeltaE     float64 `json:"colorDeltaE"`
	MoisturePercent float64 `json:"moisturePercent"`
	ResidueBaseline float64 `json:"residueBaseline"`
	TemperatureC    float64 `json:"temperatureC"`
	HumidityPercent float64 `json:"humidityPercent"`
	LightLux        float64 `json:"lightLux"`
}

type BaselineImpact struct {
	PreviousRevisionID string        `json:"previousRevisionId"`
	CurrentRevisionID  string        `json:"currentRevisionId"`
	Delta              BaselineDelta `json:"delta"`
	Summary            string        `json:"summary"`
}

type PermitValidity string

const (
	PermitValid        PermitValidity = "valid"
	PermitExpiringSoon PermitValidity = "expiring_soon"
	PermitExpired      PermitValidity = "expired"
	PermitMismatch     PermitValidity = "digest_mismatch"
)

type CaseQueueSummary struct {
	Case            ConservationCase  `json:"case"`
	CurrentRevision int               `json:"currentRevision"`
	Readiness       WorkflowReadiness `json:"readiness"`
	OpenBlockers    int               `json:"openBlockers"`
	MissingEvidence []string          `json:"missingEvidence"`
	PermitValidity  PermitValidity    `json:"permitValidity,omitempty"`
	PermitExpiresAt *time.Time        `json:"permitExpiresAt,omitempty"`
}

type CaseQueue struct {
	Items        []CaseQueueSummary `json:"items"`
	NextCursor   string             `json:"nextCursor,omitempty"`
	Total        int                `json:"total"`
	StatusCounts map[CaseStatus]int `json:"statusCounts"`
}

type CandidateMetric struct {
	Code                   string  `json:"code"`
	WorstMargin            float64 `json:"worstMargin"`
	WorstControlDifference float64 `json:"worstControlDifference"`
	Passed                 bool    `json:"passed"`
}

type CandidateComparison struct {
	ProtocolRevisionID string            `json:"protocolRevisionId"`
	ProtocolID         string            `json:"protocolId"`
	RevisionNo         int               `json:"revisionNo"`
	CoveredPairs       int               `json:"coveredPairs"`
	RequiredPairs      int               `json:"requiredPairs"`
	CoveragePercent    float64           `json:"coveragePercent"`
	Metrics            []CandidateMetric `json:"metrics"`
	TrendConclusion    string            `json:"trendConclusion"`
	OpenBlockers       int               `json:"openBlockers"`
	Gaps               []string          `json:"gaps"`
	Eligible           bool              `json:"eligible"`
	FactsDigest        string            `json:"factsDigest"`
	Digest             string            `json:"digest"`
}

type CandidateComparisonSet struct {
	Candidates []CandidateComparison `json:"candidates"`
	Digest     string                `json:"digest"`
}

type CandidateSelection struct {
	SelectionID        string    `json:"selectionId"`
	CaseID             string    `json:"caseId"`
	ProtocolRevisionID string    `json:"protocolRevisionId"`
	ComparisonDigest   string    `json:"comparisonDigest"`
	Reason             string    `json:"reason"`
	ReviewerID         string    `json:"reviewerId"`
	SelectedAt         time.Time `json:"selectedAt"`
	Valid              bool      `json:"valid"`
	InvalidReason      string    `json:"invalidReason,omitempty"`
}

type RemediationStatus string

const (
	RemediationPending  RemediationStatus = "pending"
	RemediationRetest   RemediationStatus = "awaiting_retest"
	RemediationResolved RemediationStatus = "resolved"
)

type ParameterPlan struct {
	Concentration  *float64 `json:"concentration,omitempty"`
	ContactSeconds *int     `json:"contactSeconds,omitempty"`
}

type RemediationTask struct {
	TaskID                   string            `json:"taskId"`
	CaseID                   string            `json:"caseId"`
	FindingID                string            `json:"findingId"`
	RuleCode                 string            `json:"ruleCode"`
	SourceProtocolRevisionID string            `json:"sourceProtocolRevisionId"`
	Assignee                 string            `json:"assignee"`
	Analysis                 string            `json:"analysis"`
	PlannedParameters        ParameterPlan     `json:"plannedParameters"`
	RequiredZoneIDs          []string          `json:"requiredZoneIds"`
	DueAt                    time.Time         `json:"dueAt"`
	Status                   RemediationStatus `json:"status"`
	RetestProtocolRevisionID string            `json:"retestProtocolRevisionId,omitempty"`
	ResolvedObservationIDs   []string          `json:"resolvedObservationIds,omitempty"`
	ResolvedSnapshotIDs      []string          `json:"resolvedSnapshotIds,omitempty"`
	ReturnReason             string            `json:"returnReason,omitempty"`
	CreatedAt                time.Time         `json:"createdAt"`
	UpdatedAt                time.Time         `json:"updatedAt"`
}

type VerificationCheck struct {
	Code      string `json:"code"`
	Reference string `json:"reference,omitempty"`
	Passed    bool   `json:"passed"`
	Reason    string `json:"reason"`
}

type FrozenProtocolEvidence struct {
	Protocol     CleaningProtocolRevision  `json:"protocol"`
	Observations []TrialObservation        `json:"observations"`
	References   []FrozenEvidenceReference `json:"references"`
}

type PermitReceipt struct {
	ReceiptDigest  string                 `json:"receiptDigest"`
	VerifiedAt     time.Time              `json:"verifiedAt"`
	Valid          bool                   `json:"valid"`
	Message        string                 `json:"message"`
	Validity       PermitValidity         `json:"validity"`
	RemainingDays  int                    `json:"remainingDays"`
	Permit         *WorkPermit            `json:"permit,omitempty"`
	Manifest       *FrozenManifest        `json:"manifest,omitempty"`
	FrozenEvidence FrozenProtocolEvidence `json:"frozenEvidence"`
	Checks         []VerificationCheck    `json:"checks"`
}
