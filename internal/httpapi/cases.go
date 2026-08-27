package httpapi

import (
	"net/http"
	"strconv"

	"mural-conservation-gate/internal/domain"
	"mural-conservation-gate/internal/workflow"
)

func (s *Server) ListCasesHandler(w http.ResponseWriter, r *http.Request) {
	allowed := map[string]bool{"status": true, "keyword": true, "cursor": true, "limit": true}
	for key := range r.URL.Query() {
		if !allowed[key] {
			mapError(w, domain.Invalid(key, "未知查询参数"), 0)
			return
		}
	}
	query := workflow.ListCasesQuery{Status: domain.CaseStatus(r.URL.Query().Get("status")), Keyword: r.URL.Query().Get("keyword"), KeywordProvided: r.URL.Query().Has("keyword"), Cursor: r.URL.Query().Get("cursor")}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			mapError(w, domain.Invalid("limit", "分页大小必须是整数"), 0)
			return
		}
		query.Limit = value
	}
	value, err := s.workflow.ListCases(r.Context(), query)
	if err != nil {
		mapError(w, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) CreateCaseHandler(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.CreateCaseCommand
	body, err := decodeStrict(r, &cmd)
	if err != nil {
		mapError(w, err, 0)
		return
	}
	s.idempotent(w, r, body, func() (any, int, error) {
		value, err := s.workflow.CreateCase(r.Context(), cmd)
		return value, http.StatusCreated, err
	})
}

func (s *Server) GetCaseHandler(w http.ResponseWriter, r *http.Request) {
	value, err := s.workflow.GetCase(r.Context(), r.PathValue("caseID"))
	if err != nil {
		mapError(w, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) AuditHandler(w http.ResponseWriter, r *http.Request) {
	value, err := s.workflow.Audit(r.Context(), r.PathValue("caseID"))
	if err != nil {
		mapError(w, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": value, "integrity": s.cachedAuditIntegrity(value)})
}

func (s *Server) cachedAuditIntegrity(events []domain.AuditEvent) domain.AuditIntegrity {
	key := len(events)
	if cached, ok := s.auditIntegrityCache[key]; ok {
		return cached
	}
	integrity := domain.VerifyAuditTimeline(events)
	s.auditIntegrityCache[key] = integrity
	return integrity
}
