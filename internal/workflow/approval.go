package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"mural-conservation-gate/internal/domain"
	"mural-conservation-gate/internal/evaluation"
	"mural-conservation-gate/internal/store"
)

func (s *Service) Freeze(ctx context.Context, caseID string, cmd FreezeCommand) (domain.CaseView, error) {
	return s.mutate(ctx, caseID, cmd.ExpectedVersion, func(tx *store.Tx, view domain.CaseView) (domain.CaseStatus, string, error) {
		if err := ensureState(view.Case, domain.StatusReview); err != nil {
			return "", "", err
		}
		if strings.TrimSpace(cmd.ReviewerID) == "" || strings.TrimSpace(cmd.ReviewNote) == "" {
			return "", "", domain.Invalid("reviewerId", "复核人和复核意见不能为空")
		}
		protocol, ok := findProtocol(view, cmd.ProtocolRevisionID)
		if !ok {
			return "", "", domain.Invalid("protocolRevisionId", "候选方案修订不存在")
		}
		for _, newer := range view.Protocols {
			if newer.ProtocolID == protocol.ProtocolID && newer.RevisionNo > protocol.RevisionNo {
				return "", "", domain.Invalid("protocolRevisionId", "只能冻结候选方案的最新修订")
			}
		}
		comparison := evaluation.CompareCandidates(view)
		if view.Selection == nil || view.Selection.ProtocolRevisionID != protocol.RevisionID || view.Selection.ComparisonDigest != comparison.Digest {
			return "", "", fmt.Errorf("%w: 只能冻结当前唯一入围且比选摘要仍有效的方案修订", domain.ErrIncomplete)
		}
		eligible := false
		for _, candidate := range comparison.Candidates {
			if candidate.ProtocolRevisionID == protocol.RevisionID && candidate.Eligible {
				eligible = true
			}
		}
		if !eligible {
			return "", "", fmt.Errorf("%w: 入围方案证据或风险状态已变化，需要重新比选", domain.ErrIncomplete)
		}
		if len(currentOpenFindings(view.Findings)) > 0 {
			return "", "", fmt.Errorf("%w: 仍有未关闭风险阻断项", domain.ErrIncomplete)
		}
		observations := []domain.TrialObservation{}
		hasTrial, hasControl := false, false
		zones := map[string]domain.ZoneType{}
		for _, z := range view.Zones {
			zones[z.ZoneID] = z.ZoneType
		}
		for _, item := range view.Observations {
			if item.ProtocolRevisionID == protocol.RevisionID && item.BaselineRevisionID == view.Case.BaselineRevisionID {
				observations = append(observations, item)
				hasTrial = hasTrial || zones[item.ZoneID] == domain.ZoneTrial
				hasControl = hasControl || zones[item.ZoneID] == domain.ZoneControl
			}
		}
		if !hasTrial || !hasControl {
			return "", "", fmt.Errorf("%w: 冻结方案必须同时包含试验区和对照区证据", domain.ErrIncomplete)
		}
		sort.Slice(observations, func(i, j int) bool {
			if observations[i].RoundNo == observations[j].RoundNo {
				return observations[i].ObservationID < observations[j].ObservationID
			}
			return observations[i].RoundNo < observations[j].RoundNo
		})
		canonical, err := domain.CanonicalManifest(protocol, observations)
		if err != nil {
			return "", "", err
		}
		protocolData, err := json.Marshal(protocol)
		if err != nil {
			return "", "", err
		}
		manifest := domain.FrozenManifest{ManifestID: newID("manifest"), CaseID: caseID, ProtocolRevisionID: protocol.RevisionID, ProtocolDigest: domain.DigestBytes(protocolData), ReviewerID: cmd.ReviewerID, ReviewNote: cmd.ReviewNote, ComparisonDigest: view.Selection.ComparisonDigest, SelectionReason: view.Selection.Reason, FrozenAt: s.now()}
		for _, item := range observations {
			manifest.ObservationIDs = append(manifest.ObservationIDs, item.ObservationID)
			manifest.EvidenceDigests = append(manifest.EvidenceDigests, item.EvidenceDigest)
			manifest.EvidenceReferences = append(manifest.EvidenceReferences, domain.FrozenEvidenceReference{ObservationID: item.ObservationID, EvidenceDigest: item.EvidenceDigest, ZoneID: item.ZoneID, ZoneType: zones[item.ZoneID], RoundNo: item.RoundNo})
		}
		referenceData, err := json.Marshal(manifest.EvidenceReferences)
		if err != nil {
			return "", "", err
		}
		manifest.Digest = domain.DigestBytes(append(append(canonical, '\n'), referenceData...))
		if err = tx.InsertManifest(ctx, manifest); err != nil {
			return "", "", err
		}
		if err = tx.AppendAudit(ctx, caseID, "case.frozen", cmd.ReviewerID, "复核通过并冻结唯一候选方案及证据清单", manifest); err != nil {
			return "", "", err
		}
		return domain.StatusFrozen, "", nil
	})
}

func (s *Service) IssuePermit(ctx context.Context, caseID string, cmd PermitCommand) (domain.CaseView, error) {
	unlock := s.locks.lock(caseID)
	defer unlock()
	view, err := s.store.GetView(ctx, caseID)
	if err != nil {
		return view, err
	}
	if err = requireExpected(view.Case.Version, cmd.ExpectedVersion); err != nil {
		return view, err
	}
	if err = ensureState(view.Case, domain.StatusFrozen); err != nil {
		return view, err
	}
	if view.Manifest == nil {
		return view, fmt.Errorf("%w: 缺少冻结清单", domain.ErrIncomplete)
	}
	if strings.TrimSpace(cmd.Scope) == "" || strings.TrimSpace(cmd.IssuedBy) == "" {
		return view, domain.Invalid("scope", "许可范围和签发人不能为空")
	}
	if !cmd.ExpiresAt.After(s.now()) {
		return view, domain.Invalid("expiresAt", "许可到期时间必须晚于当前时间")
	}
	now := s.now()
	permit := domain.WorkPermit{PermitID: newID("permit"), PermitNumber: fmt.Sprintf("MCG-%s-%s", now.Format("20060102"), strings.ToUpper(newID("")[1:9])), CaseID: caseID, FrozenProtocolRevisionID: view.Manifest.ProtocolRevisionID, EvidenceManifestDigest: view.Manifest.Digest, Scope: cmd.Scope, Restrictions: cmd.Restrictions, IssuedBy: cmd.IssuedBy, IssuedAt: now, ExpiresAt: cmd.ExpiresAt}
	permit.VerificationDigest = domain.PermitVerificationDigest(permit)
	tx, err := s.store.Begin(ctx)
	if err != nil {
		return view, err
	}
	defer tx.Rollback()
	if err = tx.InsertPermit(ctx, permit); err != nil {
		return view, err
	}
	if err = tx.AdvanceCase(ctx, &view.Case, cmd.ExpectedVersion, domain.StatusPermitted, ""); err != nil {
		return view, err
	}
	if err = tx.AppendAudit(ctx, caseID, "permit.issued", cmd.IssuedBy, "基于冻结快照签发正式施工许可", permit); err != nil {
		return view, err
	}
	if err = tx.Commit(); err != nil {
		return view, err
	}
	return s.GetCase(ctx, caseID)
}

func (s *Service) VerifyPermit(ctx context.Context, number string) (Verification, error) {
	number = strings.TrimSpace(number)
	if number == "" {
		return Verification{}, domain.Invalid("permitNumber", "许可编号不得为空")
	}
	permit, err := s.store.GetPermitByNumber(context.WithoutCancel(ctx), number)
	if err != nil {
		return Verification{}, err
	}
	manifest, err := s.store.GetManifest(context.WithoutCancel(ctx), permit.CaseID)
	if err != nil {
		return Verification{}, err
	}
	now := s.now()
	receipt := Verification{VerifiedAt: now, Permit: permit, Manifest: manifest, Checks: []domain.VerificationCheck{}}
	permitDigestOK := domain.PermitVerificationDigest(*permit) == permit.VerificationDigest
	receipt.Checks = append(receipt.Checks, domain.VerificationCheck{Code: "PERMIT_DIGEST", Reference: permit.PermitNumber, Passed: permitDigestOK, Reason: chooseReason(permitDigestOK, "许可字段摘要一致", "许可字段摘要不一致")})
	protocol, protocolErr := s.store.GetProtocolRevision(context.WithoutCancel(ctx), manifest.ProtocolRevisionID)
	protocolData, _ := json.Marshal(protocol)
	protocolOK := protocolErr == nil && protocol.RevisionID == permit.FrozenProtocolRevisionID && domain.DigestBytes(protocolData) == manifest.ProtocolDigest
	receipt.Checks = append(receipt.Checks, domain.VerificationCheck{Code: "PROTOCOL_REVISION", Reference: manifest.ProtocolRevisionID, Passed: protocolOK, Reason: chooseReason(protocolOK, "冻结方案修订存在且标识一致", "冻结方案修订缺失或标识不一致")})
	observations := []domain.TrialObservation{}
	evidenceOK := true
	digestByID := map[string]string{}
	for i, id := range manifest.ObservationIDs {
		if i < len(manifest.EvidenceDigests) {
			digestByID[id] = manifest.EvidenceDigests[i]
		}
		observation, loadErr := s.store.GetObservation(context.WithoutCancel(ctx), id)
		passed := loadErr == nil && observation.EvidenceDigest == digestByID[id] && observation.ProtocolRevisionID == manifest.ProtocolRevisionID
		reason := chooseReason(passed, "观察证据摘要一致", "观察证据缺失、摘要或方案修订不一致")
		receipt.Checks = append(receipt.Checks, domain.VerificationCheck{Code: "OBSERVATION_EVIDENCE", Reference: id, Passed: passed, Reason: reason})
		if passed {
			observations = append(observations, observation)
		} else {
			evidenceOK = false
		}
	}
	manifestOK := false
	if protocolErr == nil && evidenceOK {
		canonical, canonicalErr := domain.CanonicalManifest(protocol, observations)
		referenceData, referenceErr := json.Marshal(manifest.EvidenceReferences)
		manifestOK = canonicalErr == nil && referenceErr == nil && domain.DigestBytes(append(append(canonical, '\n'), referenceData...)) == manifest.Digest && manifest.Digest == permit.EvidenceManifestDigest
	}
	receipt.Checks = append(receipt.Checks, domain.VerificationCheck{Code: "FROZEN_MANIFEST", Reference: manifest.ManifestID, Passed: manifestOK, Reason: chooseReason(manifestOK, "冻结清单摘要及证据集合一致", "冻结清单摘要、方案或证据集合不一致")})
	receipt.FrozenEvidence = domain.FrozenProtocolEvidence{Protocol: protocol, Observations: observations, References: manifest.EvidenceReferences}
	receipt.Validity = permitValidity(*permit, now)
	if !permitDigestOK || !protocolOK || !evidenceOK || !manifestOK {
		receipt.Validity = domain.PermitMismatch
	}
	receipt.Valid = receipt.Validity == domain.PermitValid || receipt.Validity == domain.PermitExpiringSoon
	if now.Before(permit.ExpiresAt) {
		receipt.RemainingDays = int(math.Ceil(permit.ExpiresAt.Sub(now).Hours() / 24))
	}
	switch receipt.Validity {
	case domain.PermitValid:
		receipt.Message = "许可有效，冻结方案与全部观察证据摘要一致"
	case domain.PermitExpiringSoon:
		receipt.Message = "许可即将到期，冻结方案与全部观察证据摘要一致"
	case domain.PermitExpired:
		receipt.Message = "冻结证据完整，但许可已到期，不可用于施工"
	default:
		receipt.Message = "许可或冻结证据摘要不一致，整体验真无效"
	}
	canonicalReceipt := struct {
		PermitNumber string                     `json:"permitNumber"`
		VerifiedAt   time.Time                  `json:"verifiedAt"`
		Checks       []domain.VerificationCheck `json:"checks"`
		Validity     domain.PermitValidity      `json:"validity"`
	}{permit.PermitNumber, now, receipt.Checks, receipt.Validity}
	data, _ := json.Marshal(canonicalReceipt)
	receipt.ReceiptDigest = domain.DigestBytes(data)
	tx, beginErr := s.store.Begin(context.WithoutCancel(ctx))
	if beginErr != nil {
		return receipt, beginErr
	}
	defer tx.Rollback()
	if err = tx.AppendAudit(context.WithoutCancel(ctx), permit.CaseID, "permit.verified", "permit-verifier", "生成许可验真回执并逐项核对冻结证据", receipt); err != nil {
		return receipt, err
	}
	if err = tx.Commit(); err != nil {
		return receipt, err
	}
	return receipt, nil
}

func chooseReason(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}
