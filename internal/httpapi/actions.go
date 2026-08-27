package httpapi

import (
	"net/http"
	"strconv"

	"mural-conservation-gate/internal/domain"

	"mural-conservation-gate/internal/workflow"
)

func (s *Server) SubmitBaselineHandler(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.BaselineCommand
	s.writeCaseAction(w, r, &cmd, func() (any, error) { return s.workflow.SubmitBaseline(r.Context(), r.PathValue("caseID"), cmd) })
}

func (s *Server) AddZoneHandler(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.ZoneCommand
	s.writeCaseAction(w, r, &cmd, func() (any, error) { return s.workflow.AddZone(r.Context(), r.PathValue("caseID"), cmd) })
}

func (s *Server) ReviseProtocolHandler(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.ProtocolCommand
	s.writeCaseAction(w, r, &cmd, func() (any, error) { return s.workflow.ReviseProtocol(r.Context(), r.PathValue("caseID"), cmd) })
}

func (s *Server) SubmitObservationHandler(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.ObservationCommand
	s.writeCaseAction(w, r, &cmd, func() (any, error) { return s.workflow.SubmitObservation(r.Context(), r.PathValue("caseID"), cmd) })
}

func (s *Server) SubmitPairedObservationsHandler(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.PairedObservationBatchCommand
	s.writeCaseAction(w, r, &cmd, func() (any, error) {
		return s.workflow.SubmitPairedObservations(r.Context(), r.PathValue("caseID"), cmd)
	})
}

func (s *Server) CandidateComparisonsHandler(w http.ResponseWriter, r *http.Request) {
	value, err := s.workflow.CandidateComparisons(r.Context(), r.PathValue("caseID"))
	if err != nil {
		mapError(w, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) SelectCandidateHandler(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.SelectionCommand
	s.writeCaseAction(w, r, &cmd, func() (any, error) { return s.workflow.SelectCandidate(r.Context(), r.PathValue("caseID"), cmd) })
}

func (s *Server) CreateRemediationHandler(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.RemediationCommand
	s.writeCaseAction(w, r, &cmd, func() (any, error) { return s.workflow.CreateRemediation(r.Context(), r.PathValue("caseID"), cmd) })
}

func (s *Server) RemediationQueueHandler(w http.ResponseWriter, r *http.Request) {
	allowed := map[string]bool{"assignee": true, "overdue": true, "severity": true}
	for key := range r.URL.Query() {
		if !allowed[key] {
			mapError(w, domain.Invalid(key, "未知查询参数"), 0)
			return
		}
	}
	overdue := false
	if raw := r.URL.Query().Get("overdue"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			mapError(w, domain.Invalid("overdue", "逾期条件必须是 true 或 false"), 0)
			return
		}
		overdue = value
	}
	values, err := s.workflow.RemediationQueue(r.Context(), r.URL.Query().Get("assignee"), overdue, domain.Severity(r.URL.Query().Get("severity")))
	if err != nil {
		mapError(w, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) FreezeHandler(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.FreezeCommand
	s.writeCaseAction(w, r, &cmd, func() (any, error) { return s.workflow.Freeze(r.Context(), r.PathValue("caseID"), cmd) })
}

func (s *Server) IssuePermitHandler(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.PermitCommand
	s.writeCaseAction(w, r, &cmd, func() (any, error) { return s.workflow.IssuePermit(r.Context(), r.PathValue("caseID"), cmd) })
}

func (s *Server) writeCaseAction(w http.ResponseWriter, r *http.Request, target any, action func() (any, error)) {
	body, err := decodeStrict(r, target)
	if err != nil {
		mapError(w, err, 0)
		return
	}
	s.idempotent(w, r, body, func() (any, int, error) { value, err := action(); return value, http.StatusOK, err })
}

func (s *Server) VerifyPermitHandler(w http.ResponseWriter, r *http.Request) {
	value, err := s.workflow.VerifyPermit(r.Context(), r.PathValue("permitNumber"))
	if err != nil {
		mapError(w, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
