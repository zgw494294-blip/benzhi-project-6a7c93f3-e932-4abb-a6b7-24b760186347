package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
	"time"

	"mural-conservation-gate/internal/domain"
	"mural-conservation-gate/internal/workflow"
)

//go:embed web/*
var webFiles embed.FS

type Server struct {
	workflow            *workflow.Service
	mux                 *http.ServeMux
	auditIntegrityCache map[int]domain.AuditIntegrity
}

func New(service *workflow.Service) *Server {
	s := &Server{
		workflow:            service,
		mux:                 http.NewServeMux(),
		auditIntegrityCache: map[int]domain.AuditIntegrity{},
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.HealthHandler)
	s.mux.HandleFunc("GET /", s.WorkbenchHandler)
	static, _ := fs.Sub(webFiles, "web")
	s.mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(static))))
	s.mux.HandleFunc("POST /api/cases", s.CreateCaseHandler)
	s.mux.HandleFunc("GET /api/cases", s.ListCasesHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}", s.GetCaseHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/audit", s.AuditHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/baseline", s.SubmitBaselineHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/zones", s.AddZoneHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/protocols", s.ReviseProtocolHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/observations", s.SubmitObservationHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/observation-batches", s.SubmitPairedObservationsHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/observations/batch", s.SubmitPairedObservationsHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/candidates", s.CandidateComparisonsHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/candidate-selection", s.SelectCandidateHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/remediations", s.CreateRemediationHandler)
	s.mux.HandleFunc("GET /api/remediations", s.RemediationQueueHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/freeze", s.FreezeHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/permit", s.IssuePermitHandler)
	s.mux.HandleFunc("GET /api/permits/{permitNumber}/verify", s.VerifyPermitHandler)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.workflow.Store().Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "storage_unavailable", "持久化服务不可用", err, 0)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func (s *Server) WorkbenchHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "工作台资源不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
