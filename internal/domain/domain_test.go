package domain

import (
	"testing"
	"time"
)

func TestZonesOverlapAndPermitDigest(t *testing.T) {
	a := TrialZone{BoundaryPoints: []Point{{0, 0}, {10, 0}, {10, 10}, {0, 10}}}
	b := TrialZone{BoundaryPoints: []Point{{20, 0}, {30, 0}, {30, 10}, {20, 10}}}
	if ZonesOverlap(a, b) {
		t.Fatal("分离分区不应重叠")
	}
	b.BoundaryPoints = []Point{{5, 5}, {15, 5}, {15, 15}, {5, 15}}
	if !ZonesOverlap(a, b) {
		t.Fatal("交叉分区应判为重叠")
	}
	now := time.Now().UTC()
	manifest := FrozenManifest{ProtocolRevisionID: "rev", Digest: "manifest"}
	permit := WorkPermit{PermitNumber: "MCG-1", CaseID: "case", FrozenProtocolRevisionID: "rev", EvidenceManifestDigest: "manifest", Scope: "测试", Restrictions: []string{"限制"}, IssuedBy: "负责人", IssuedAt: now, ExpiresAt: now.Add(time.Hour)}
	permit.VerificationDigest = PermitVerificationDigest(permit)
	if ok, _ := VerifyPermit(permit, manifest, now); !ok {
		t.Fatal("合法许可应通过校验")
	}
	permit.Scope = "篡改"
	if ok, _ := VerifyPermit(permit, manifest, now); ok {
		t.Fatal("篡改许可不得通过校验")
	}
}
