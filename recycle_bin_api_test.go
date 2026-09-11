package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irains/fileharbor/conf"
)

func TestRecycleBinAPIUsesSessionCSRFAndAudit(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})
	if err := os.WriteFile(filepath.Join(conf.FileHarbor, "notes.txt"), []byte("contents"), 0644); err != nil {
		t.Fatal(err)
	}
	manager := testManager(t)
	state := newTestState(t)
	router := newRouter(manager, state)
	cookie := loginCookie(t, router)
	csrf := sessionCSRF(t, manager, cookie)

	beforeAudit := len(readAuditEvents(t, state))
	request := httptest.NewRequest(http.MethodPost, "/do/rm", strings.NewReader(url.Values{"path": {"notes.txt"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-CSRF-Token", csrf)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("trash move = %d: %s", response.Code, response.Body.String())
	}
	var moved struct {
		Entry TrashEntry `json:"entry"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &moved); err != nil || moved.Entry.ID == "" || moved.Entry.OriginalPath != "notes.txt" {
		t.Fatalf("trash move response = %s, %v", response.Body.String(), err)
	}
	if _, err := os.Lstat(filepath.Join(conf.FileHarbor, "notes.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source was not moved to recycle bin: %v", err)
	}
	events := readAuditEvents(t, state)
	if len(events) != beforeAudit+2 || events[len(events)-2].Event != "file.trash" || events[len(events)-2].Outcome != "attempted" || events[len(events)-1].Event != "file.trash" || events[len(events)-1].Outcome != "success" || events[len(events)-1].Path != "notes.txt" {
		t.Fatalf("trash audit = %#v", events[beforeAudit:])
	}

	request = httptest.NewRequest(http.MethodGet, "/api/trash", nil)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"original_path":"notes.txt"`) {
		t.Fatalf("trash listing = %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/trash/"+moved.Entry.ID+"/purge", strings.NewReader(`{"confirmation":"DELETE"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "csrf_invalid") {
		t.Fatalf("purge missing csrf = %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/trash/"+moved.Entry.ID+"/restore", nil)
	request.Header.Set("X-CSRF-Token", csrf)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("restore = %d: %s", response.Code, response.Body.String())
	}
	if data, err := os.ReadFile(filepath.Join(conf.FileHarbor, "notes.txt")); err != nil || string(data) != "contents" {
		t.Fatalf("restored file = %q, %v", data, err)
	}

	bearer := httptest.NewRequest(http.MethodGet, "/api/trash", nil)
	bearer.Header.Set("Authorization", "Bearer abcdef0123456789abcdef0123456789")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, bearer)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("trash bearer access = %d: %s", response.Code, response.Body.String())
	}
}

func TestRecycleBinPermanentActionsRequireExactConfirmation(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})
	if err := os.WriteFile(filepath.Join(conf.FileHarbor, "notes.txt"), []byte("contents"), 0644); err != nil {
		t.Fatal(err)
	}
	manager := testManager(t)
	state := newTestState(t)
	router := newRouter(manager, state)
	cookie := loginCookie(t, router)
	csrf := sessionCSRF(t, manager, cookie)

	move := httptest.NewRequest(http.MethodPost, "/do/rm", strings.NewReader(url.Values{"path": {"notes.txt"}}.Encode()))
	move.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	move.Header.Set("X-CSRF-Token", csrf)
	move.AddCookie(cookie)
	movedResponse := httptest.NewRecorder()
	router.ServeHTTP(movedResponse, move)
	if movedResponse.Code != http.StatusOK {
		t.Fatalf("trash move = %d: %s", movedResponse.Code, movedResponse.Body.String())
	}
	var moved struct {
		Entry TrashEntry `json:"entry"`
	}
	if err := json.Unmarshal(movedResponse.Body.Bytes(), &moved); err != nil {
		t.Fatal(err)
	}

	purge := httptest.NewRequest(http.MethodPost, "/api/trash/"+moved.Entry.ID+"/purge", strings.NewReader(`{"confirmation":"delete"}`))
	purge.Header.Set("Content-Type", "application/json")
	purge.Header.Set("X-CSRF-Token", csrf)
	purge.AddCookie(cookie)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, purge)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "confirmation_required") {
		t.Fatalf("non-exact purge confirmation = %d: %s", response.Code, response.Body.String())
	}

	empty := httptest.NewRequest(http.MethodPost, "/api/trash/empty", strings.NewReader(`{"confirmation":"DELETE","extra":true}`))
	empty.Header.Set("Content-Type", "application/json")
	empty.Header.Set("X-CSRF-Token", csrf)
	empty.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, empty)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_path") {
		t.Fatalf("unknown confirmation field = %d: %s", response.Code, response.Body.String())
	}

	purge = httptest.NewRequest(http.MethodPost, "/api/trash/"+moved.Entry.ID+"/purge", strings.NewReader(`{"confirmation":"DELETE"}`))
	purge.Header.Set("Content-Type", "application/json")
	purge.Header.Set("X-CSRF-Token", csrf)
	purge.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, purge)
	if response.Code != http.StatusOK {
		t.Fatalf("exact purge confirmation = %d: %s", response.Code, response.Body.String())
	}
}
