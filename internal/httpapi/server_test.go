package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mural-conservation-gate/internal/store"
	"mural-conservation-gate/internal/workflow"
)

func TestWorkbenchAndStrictJSON(t *testing.T) {
	repository, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	handler := New(workflow.New(repository)).Handler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<body>") {
		t.Fatal("工作台 HTML 不完整")
	}
	req = httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader(`{"siteName":"x","unknown":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "strict-test")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 422 {
		t.Fatalf("未知字段应被拒绝，实际 %d", rec.Code)
	}
}
