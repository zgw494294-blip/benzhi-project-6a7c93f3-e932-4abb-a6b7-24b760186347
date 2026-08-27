package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

func CanModify(status CaseStatus) error {
	if status == StatusFrozen || status == StatusPermitted {
		return ErrFrozen
	}
	return nil
}

func CanTransition(from, to CaseStatus) bool {
	allowed := map[CaseStatus]map[CaseStatus]bool{
		StatusDraft:       {StatusTesting: true},
		StatusTesting:     {StatusTesting: true, StatusRemediation: true, StatusReview: true},
		StatusRemediation: {StatusRemediation: true, StatusTesting: true, StatusReview: true},
		StatusReview:      {StatusTesting: true, StatusRemediation: true, StatusFrozen: true},
		StatusFrozen:      {StatusPermitted: true},
		StatusPermitted:   {},
	}
	return allowed[from][to]
}

func RequireTransition(from, to CaseStatus) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidState, from, to)
	}
	return nil
}

func CanonicalManifest(protocol CleaningProtocolRevision, observations []TrialObservation) ([]byte, error) {
	sorted := append([]TrialObservation(nil), observations...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].RoundNo == sorted[j].RoundNo {
			return sorted[i].ObservationID < sorted[j].ObservationID
		}
		return sorted[i].RoundNo < sorted[j].RoundNo
	})
	type evidence struct {
		ID      string `json:"id"`
		ZoneID  string `json:"zoneId"`
		RoundNo int    `json:"roundNo"`
		Digest  string `json:"digest"`
	}
	record := struct {
		Protocol CleaningProtocolRevision `json:"protocol"`
		Evidence []evidence               `json:"evidence"`
	}{Protocol: protocol}
	for _, observation := range sorted {
		record.Evidence = append(record.Evidence, evidence{ID: observation.ObservationID, ZoneID: observation.ZoneID, RoundNo: observation.RoundNo, Digest: observation.EvidenceDigest})
	}
	return json.Marshal(record)
}

func DigestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func PermitVerificationDigest(permit WorkPermit) string {
	parts := []string{
		permit.PermitNumber,
		permit.CaseID,
		permit.FrozenProtocolRevisionID,
		permit.EvidenceManifestDigest,
		permit.Scope,
		strings.Join(permit.Restrictions, "\x1f"),
		permit.IssuedBy,
		permit.IssuedAt.UTC().Format(time.RFC3339Nano),
		permit.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}
	return DigestBytes([]byte(strings.Join(parts, "\x1e")))
}

func VerifyPermit(permit WorkPermit, manifest FrozenManifest, now time.Time) (bool, string) {
	if permit.EvidenceManifestDigest != manifest.Digest || permit.FrozenProtocolRevisionID != manifest.ProtocolRevisionID {
		return false, "许可与冻结证据清单不一致"
	}
	if PermitVerificationDigest(permit) != permit.VerificationDigest {
		return false, "许可校验摘要不匹配"
	}
	if !now.Before(permit.ExpiresAt) {
		return false, "许可已到期"
	}
	return true, "许可有效且校验摘要一致"
}
