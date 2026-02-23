package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"scaffold/db"
)

func TestHandleDomainsReturnsDriftStates(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/domains", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got []domainResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(got) < 2 {
		t.Fatalf("expected at least 2 entries (seeded domains + dump), got %d", len(got))
	}

	last := got[len(got)-1]
	if last.Name != "The Dump" {
		t.Fatalf("expected last entry to be The Dump, got %q", last.Name)
	}
	if last.ID != 0 {
		t.Fatalf("expected dump id=0, got %d", last.ID)
	}
	if last.DriftState != "cold" {
		t.Fatalf("expected dump drift_state=cold, got %q", last.DriftState)
	}

	for _, d := range got[:len(got)-1] {
		if d.DriftState == "" {
			t.Fatalf("domain %q has empty drift_state", d.Name)
		}
		if d.DriftLabel == "" {
			t.Fatalf("domain %q has empty drift_label", d.Name)
		}
	}
}

func TestHandleDomainDetailReturnsAggregatedView(t *testing.T) {
	srv, database := newTestServer(t)

	domains, err := database.ListDomains()
	if err != nil || len(domains) == 0 {
		t.Fatal("expected seeded domains")
	}
	domain := domains[0]
	briefing := "briefing text from api test"
	if err := database.UpdateDomain(domain.ID, db.DomainUpdateOpts{Briefing: &briefing}); err != nil {
		t.Fatalf("update domain briefing: %v", err)
	}

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/domains/"+strconv.Itoa(domain.ID), ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got domainDetailResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.Domain.ID != domain.ID {
		t.Fatalf("expected domain id %d, got %d", domain.ID, got.Domain.ID)
	}
	if got.Domain.Name != domain.Name {
		t.Fatalf("expected domain name %q, got %q", domain.Name, got.Domain.Name)
	}
	if got.Domain.Briefing != briefing {
		t.Fatalf("expected domain briefing %q, got %q", briefing, got.Domain.Briefing)
	}
	if got.DriftState == "" {
		t.Fatal("expected non-empty drift_state")
	}
}

func TestHandleDomainDetailNotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/domains/9999", ""))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleDomainPatchUpdatesFields(t *testing.T) {
	srv, database := newTestServer(t)

	domains, _ := database.ListDomains()
	if len(domains) == 0 {
		t.Fatal("expected seeded domains")
	}
	domain := domains[0]

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPatch, "/api/domains/"+strconv.Itoa(domain.ID),
		`{"status_line":"updated status","briefing":"new briefing"}`))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	updated, err := database.GetDomain(domain.ID)
	if err != nil || updated == nil {
		t.Fatal("expected domain to exist after patch")
	}
	if !updated.StatusLine.Valid || updated.StatusLine.String != "updated status" {
		t.Fatalf("expected status_line 'updated status', got %v", updated.StatusLine)
	}
	if !updated.Briefing.Valid || updated.Briefing.String != "new briefing" {
		t.Fatalf("expected briefing 'new briefing', got %v", updated.Briefing)
	}
}

func TestHandleDomainPatchImportanceOnlyDoesNotTouch(t *testing.T) {
	srv, database := newTestServer(t)

	domains, _ := database.ListDomains()
	if len(domains) == 0 {
		t.Fatal("expected seeded domains")
	}
	domain := domains[0]
	origTouch := domain.LastTouchedAt

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPatch, "/api/domains/"+strconv.Itoa(domain.ID),
		`{"importance":2}`))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	updated, _ := database.GetDomain(domain.ID)
	if updated.Importance != 2 {
		t.Fatalf("expected importance 2, got %d", updated.Importance)
	}
	if updated.LastTouchedAt != origTouch {
		t.Fatal("importance-only update should not change last_touched_at")
	}
}

func TestHandleDomainPatchMissingDomainReturnsNotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPatch, "/api/domains/999999",
		`{"status_line":"missing"}`))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleDomainPatchInvalidImportanceReturnsBadRequest(t *testing.T) {
	srv, database := newTestServer(t)

	domains, _ := database.ListDomains()
	if len(domains) == 0 {
		t.Fatal("expected seeded domains")
	}
	domain := domains[0]

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPatch, "/api/domains/"+strconv.Itoa(domain.ID),
		`{"importance":6}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleDomainPatchEmptyBodyReturnsBadRequest(t *testing.T) {
	srv, database := newTestServer(t)

	domains, _ := database.ListDomains()
	if len(domains) == 0 {
		t.Fatal("expected seeded domains")
	}
	domain := domains[0]

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPatch, "/api/domains/"+strconv.Itoa(domain.ID), `{}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleDomainsDumpReturnsUndomainedItems(t *testing.T) {
	srv, database := newTestServer(t)

	if _, err := database.InsertCapture("undomained item", "web"); err != nil {
		t.Fatalf("insert capture: %v", err)
	}

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/domains/dump", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got dumpResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(got.Captures) == 0 {
		t.Fatal("expected at least 1 dump capture")
	}
}

func TestHandleDomainsDumpCountIncludesMemoriesAndCaptures(t *testing.T) {
	srv, database := newTestServer(t)

	if _, err := database.InsertCapture("undomained capture", "web"); err != nil {
		t.Fatalf("insert capture: %v", err)
	}
	if err := database.InsertMemory(db.Memory{
		ID:         "dump-count-memory",
		Type:       "Fact",
		Content:    "undomained memory",
		Title:      "undomained memory",
		Importance: 0.3,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/domains", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got []domainResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var dump *domainResponse
	for i := range got {
		if got[i].ID == 0 && got[i].Name == "The Dump" {
			dump = &got[i]
			break
		}
	}
	if dump == nil {
		t.Fatal("expected The Dump entry in domain list")
	}
	if dump.OpenTaskCount != 2 {
		t.Fatalf("expected dump open_task_count 2, got %d", dump.OpenTaskCount)
	}
	if dump.DriftLabel != "2 items" {
		t.Fatalf("expected dump drift_label '2 items', got %q", dump.DriftLabel)
	}
}

func TestHandleDomainsDumpReturnsEmptyArraysNotNull(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/domains/dump", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, key := range []string{"captures", "memories"} {
		if string(raw[key]) == "null" {
			t.Fatalf("expected %s to be [], got null", key)
		}
	}
}
