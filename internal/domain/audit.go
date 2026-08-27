package domain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type AuditIntegrity struct {
	Continuous     bool   `json:"continuous"`
	EventCount     int    `json:"eventCount"`
	FirstSequence  int64  `json:"firstSequence,omitempty"`
	LastSequence   int64  `json:"lastSequence,omitempty"`
	TimelineDigest string `json:"timelineDigest"`
	Message        string `json:"message"`
}

func VerifyAuditTimeline(events []AuditEvent) AuditIntegrity {
	result := AuditIntegrity{Continuous: true, EventCount: len(events)}
	if len(events) == 0 {
		result.TimelineDigest = DigestBytes(nil)
		result.Message = "时间线尚无审计事件"
		return result
	}
	result.FirstSequence = events[0].Sequence
	result.LastSequence = events[len(events)-1].Sequence
	parts := make([]string, 0, len(events))
	previous := events[0].Sequence - 1
	caseID := events[0].CaseID
	for _, event := range events {
		if event.Sequence != previous+1 || event.CaseID != caseID {
			result.Continuous = false
		}
		canonical, _ := json.Marshal(struct {
			Sequence  int64  `json:"sequence"`
			CaseID    string `json:"caseId"`
			EventType string `json:"eventType"`
			ActorID   string `json:"actorId"`
			Summary   string `json:"summary"`
			CreatedAt string `json:"createdAt"`
		}{event.Sequence, event.CaseID, event.EventType, event.ActorID, event.Summary, event.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")})
		parts = append(parts, strconv.FormatInt(event.Sequence, 10)+":"+DigestBytes(canonical))
		previous = event.Sequence
	}
	result.TimelineDigest = DigestBytes([]byte(strings.Join(parts, "\n")))
	if result.Continuous {
		result.Message = fmt.Sprintf("%d 条审计事件顺序连续", len(events))
	} else {
		result.Message = "审计事件序号或案卷归属不连续"
	}
	return result
}
