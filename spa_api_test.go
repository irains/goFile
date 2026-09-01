package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irains/fileharbor/conf"
)

func TestEmbeddedSPAShellAndAssets(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, "/fileharbor"
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})

	bundle, err := loadWebAssets()
	if err != nil {
		t.Fatal(err)
	}
	entryAsset := bundle.entry.File
	handler := withBasePath(newTestRouter(t, testManager(t)))
	cookie := loginCookie(t, handler)
	request := httptest.NewRequest(http.MethodGet, "/fileharbor/", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("shell status = %d: %s", response.Code, response.Body.String())
	}
	for header, want := range map[string]string{"Cache-Control": "no-store, private", "Content-Type": "text/html; charset=utf-8"} {
		if got := response.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
	body := response.Body.String()
	if csp := response.Header().Get("Content-Security-Policy"); csp == "" || !bytes.Contains([]byte(csp), []byte("style-src 'self' 'nonce-")) {
		t.Fatalf("shell CSP = %q", csp)
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("/fileharbor/assets/"+entryAsset)) || bytes.Contains(response.Body.Bytes(), []byte("fileharbor-csrf")) || !bytes.Contains(response.Body.Bytes(), []byte("fileharbor-nonce")) || !bytes.Contains(response.Body.Bytes(), []byte("fileharbor-login-next")) {
		t.Fatalf("shell metadata or embedded asset reference is invalid: %s", response.Body.String())
	}
	if !strings.Contains(body, `localStorage.getItem("fileharbor-mode")`) || !strings.Contains(body, `data-mui-color-scheme`) || !strings.Contains(body, `nonce="`) {
		t.Fatalf("shell theme bootstrap is invalid: %s", body)
	}
	prepaint := strings.Index(body, `localStorage.getItem("fileharbor-mode")`)
	asset := strings.Index(body, "/fileharbor/assets/")
	if prepaint == -1 || asset == -1 || prepaint > asset {
		t.Fatalf("theme bootstrap must precede external assets: %s", body)
	}

	request = httptest.NewRequest(http.MethodGet, "/fileharbor/assets/"+entryAsset, nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("asset response = %d, cache %q", response.Code, response.Header().Get("Cache-Control"))
	}

	for _, privateAsset := range []string{"manifest.json", "index.html"} {
		request = httptest.NewRequest(http.MethodGet, "/fileharbor/assets/"+privateAsset, nil)
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d", privateAsset, response.Code)
		}
	}

	request = httptest.NewRequest(http.MethodGet, "/fileharbor/not-a-spa-route", nil)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d", response.Code)
	}
}

func TestEmbeddedSPALoginNextIsServerSanitized(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, "/fileharbor"
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})

	handler := withBasePath(newTestRouter(t, testManager(t)))
	request := httptest.NewRequest(http.MethodGet, "/fileharbor/login?next=https://attacker.example/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("login shell status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `fileharbor-login-next" content="/fileharbor/"`) {
		t.Fatalf("unsafe login next in shell: %s", response.Body.String())
	}
}

func TestReactSessionListingAndEditorAPIs(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})
	if err := os.WriteFile(filepath.Join(conf.FileHarbor, "notes.txt"), []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	manager := testManager(t)
	router := newTestRouter(t, manager)

	loginBody := `{"username":"admin","password":"a durable password"}`
	request := httptest.NewRequest(http.MethodPost, "/api/session/login", bytes.NewBufferString(loginBody))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("API login = %d: %s", response.Code, response.Body.String())
	}
	cookie := sessionCookieFromResponse(t, response)
	var session struct {
		OK      bool `json:"ok"`
		Session struct {
			CSRF      string `json:"csrf_token"`
			ExpiresAt string `json:"expires_at"`
		} `json:"session"`
		Capabilities struct {
			Mutate     bool `json:"mutate"`
			EditorSave bool `json:"editor_save"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil || !session.OK || session.Session.CSRF == "" || session.Session.ExpiresAt == "" || !session.Capabilities.Mutate || !session.Capabilities.EditorSave {
		t.Fatalf("session response = %s, %v", response.Body.String(), err)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/not-a-route", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || !bytes.Contains(response.Body.Bytes(), []byte(`"code":"not_found"`)) {
		t.Fatalf("API fallback = %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/listing", nil)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("listing = %d: %s", response.Code, response.Body.String())
	}
	var listing struct {
		OK        bool `json:"ok"`
		Directory struct {
			Path         string  `json:"path"`
			ParentPath   *string `json:"parent_path"`
			ListingToken string  `json:"listing_token"`
			Entries      []struct {
				Path        string `json:"path"`
				Version     string `json:"version"`
				Previewable bool   `json:"previewable"`
			} `json:"entries"`
		} `json:"directory"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listing); err != nil || !listing.OK || listing.Directory.Path != "" || listing.Directory.ParentPath != nil || listing.Directory.ListingToken == "" || len(listing.Directory.Entries) != 1 || listing.Directory.Entries[0].Path != "notes.txt" || listing.Directory.Entries[0].Version == "" || !listing.Directory.Entries[0].Previewable {
		t.Fatalf("listing response = %s, %v", response.Body.String(), err)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/editor/content?path=notes.txt", nil)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("editor GET = %d: %s", response.Code, response.Body.String())
	}
	var content struct {
		OK     bool `json:"ok"`
		Editor struct {
			Content string `json:"content"`
			Version string `json:"version"`
		} `json:"editor"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &content); err != nil || !content.OK || content.Editor.Content != "before" || content.Editor.Version == "" {
		t.Fatalf("editor content = %s, %v", response.Body.String(), err)
	}

	saveBody := `{"path":"notes.txt","content":"after","expected_version":"` + content.Editor.Version + `"}`
	request = httptest.NewRequest(http.MethodPut, "/api/editor/content", bytes.NewBufferString(saveBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", session.Session.CSRF)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("editor PUT = %d: %s", response.Code, response.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(conf.FileHarbor, "notes.txt"))
	if err != nil || string(data) != "after" {
		t.Fatalf("saved content = %q, %v", data, err)
	}

	request = httptest.NewRequest(http.MethodPut, "/api/editor/content", bytes.NewBufferString(saveBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", session.Session.CSRF)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("stale editor PUT = %d: %s", response.Code, response.Body.String())
	}
}
