package main

import (
	"bytes"
	"goFile/auth"
	"goFile/conf"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

func testManager(t *testing.T) *auth.Manager {
	t.Helper()
	manager, err := auth.NewManager(auth.Config{
		Username: "admin", Password: "a durable password",
		SessionSecret: "0123456789abcdef0123456789abcdef",
		APIToken: "abcdef0123456789abcdef0123456789",
	})
	if err != nil { t.Fatal(err) }
	return manager
}

func loginCookie(t *testing.T, router http.Handler) *http.Cookie {
	t.Helper()
	body := url.Values{"username": []string{"admin"}, "password": []string{"a durable password"}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusFound { t.Fatalf("login status = %d", response.Code) }
	if got := response.Header().Get("Cache-Control"); got != "no-store, private" { t.Fatalf("login cache-control = %q", got) }
	cookies := response.Result().Cookies()
	if len(cookies) != 1 { t.Fatal("expected session cookie") }
	return cookies[0]
}

func TestRouterRequiresAuthentication(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousTemplates := conf.GoFile, reader, uploader, templateSets
	conf.GoFile, reader, uploader = t.TempDir(), false, false
	t.Cleanup(func() { conf.GoFile, reader, uploader, templateSets = previousRoot, previousReader, previousUploader, previousTemplates })
	if err := os.WriteFile(conf.GoFile+string(os.PathSeparator)+"a.txt", []byte("a"), 0644); err != nil { t.Fatal(err) }
	router := newRouter(testManager(t))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusFound { t.Fatalf("expected redirect, got %d", response.Code) }
	cookie := loginCookie(t, router)
	request = httptest.NewRequest(http.MethodGet, "/", nil); request.AddCookie(cookie)
	response = httptest.NewRecorder(); router.ServeHTTP(response, request)
	if response.Code != http.StatusOK { t.Fatalf("authenticated list = %d", response.Code) }
	if got := response.Header().Get("Cache-Control"); got != "no-store, private" { t.Fatalf("directory cache-control = %q", got) }
}

func TestBatchDownloadRouteDoesNotConflictWithFileDownloads(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousTemplates := conf.GoFile, reader, uploader, templateSets
	conf.GoFile, reader, uploader = t.TempDir(), false, false
	t.Cleanup(func() { conf.GoFile, reader, uploader, templateSets = previousRoot, previousReader, previousUploader, previousTemplates })
	router := newRouter(testManager(t))
	request := httptest.NewRequest(http.MethodGet, "/batch-download/not-a-ticket", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("unauthenticated batch download = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/download/missing.txt", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("unauthenticated file download = %d", response.Code)
	}
}

func TestAPIUploadAllowsBearerOnly(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousTemplates := conf.GoFile, reader, uploader, templateSets
	conf.GoFile, reader, uploader = t.TempDir(), false, false
	t.Cleanup(func() { conf.GoFile, reader, uploader, templateSets = previousRoot, previousReader, previousUploader, previousTemplates })
	router := newRouter(testManager(t))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer abcdef0123456789abcdef0123456789")
	response := httptest.NewRecorder(); router.ServeHTTP(response, request)
	if response.Code != http.StatusFound { t.Fatalf("bearer must not browse, got %d", response.Code) }
}

func TestLoginRejectsUnsafeNextAndPreviewIsPlainText(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousTemplates := conf.GoFile, reader, uploader, templateSets
	conf.GoFile, reader, uploader = t.TempDir(), false, false
	t.Cleanup(func() { conf.GoFile, reader, uploader, templateSets = previousRoot, previousReader, previousUploader, previousTemplates })
	if err := os.WriteFile(conf.GoFile+string(os.PathSeparator)+"payload.html", []byte("<script>alert(1)</script>"), 0644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(conf.GoFile+string(os.PathSeparator)+"notes.txt", []byte("safe text"), 0644); err != nil { t.Fatal(err) }
	router := newRouter(testManager(t))
	body := url.Values{"username": []string{"admin"}, "password": []string{"a durable password"}, "next": []string{"/\\\\evil.example"}}.Encode()
	request := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder(); router.ServeHTTP(response, request)
	if location := response.Header().Get("Location"); location != "/" { t.Fatalf("unsafe next redirected to %q", location) }
	cookie := response.Result().Cookies()[0]
	request = httptest.NewRequest(http.MethodGet, "/view/payload.html", nil); request.AddCookie(cookie)
	response = httptest.NewRecorder(); router.ServeHTTP(response, request)
	if got := response.Header().Get("Content-Disposition"); got == "" { t.Fatal("active content preview must download") }
	if got := response.Header().Get("Content-Type"); got == "text/html; charset=utf-8" { t.Fatalf("active content type = %q", got) }
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" { t.Fatalf("nosniff = %q", got) }
	if got := response.Header().Get("Content-Security-Policy"); got != "default-src 'none'; base-uri 'none'; form-action 'none'; sandbox" { t.Fatalf("active content CSP = %q", got) }
	if got := response.Header().Get("Cache-Control"); got != "no-store, private" { t.Fatalf("cache-control = %q", got) }

	request = httptest.NewRequest(http.MethodGet, "/view/notes.txt", nil); request.AddCookie(cookie)
	response = httptest.NewRecorder(); router.ServeHTTP(response, request)
	if got := response.Header().Get("Content-Security-Policy"); got != "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; sandbox" { t.Fatalf("text preview CSP = %q", got) }

	request = httptest.NewRequest(http.MethodGet, "/view/missing.txt", nil); request.AddCookie(cookie)
	response = httptest.NewRecorder(); router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound { t.Fatalf("missing preview status = %d", response.Code) }
	if got := response.Header().Get("Cache-Control"); got != "no-store, private" { t.Fatalf("missing preview cache-control = %q", got) }
}