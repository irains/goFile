package main

import (
	"bytes"
	"goFile/auth"
	"goFile/conf"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func testManager(t *testing.T) *auth.Manager {
	t.Helper()
	passwordHash, err := auth.GeneratePasswordHash([]byte("a durable password"))
	if err != nil {
		t.Fatal(err)
	}
	manager, err := auth.NewManager(auth.Config{
		Username: "admin", PasswordHash: passwordHash,
		SessionSecret: "0123456789abcdef0123456789abcdef",
		APIToken:      "abcdef0123456789abcdef0123456789",
		CookiePath:    basePath,
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

func TestSafeNextPathUsesPublicBase(t *testing.T) {
	previousBasePath := basePath
	basePath = "/gofile"
	t.Cleanup(func() { basePath = previousBasePath })
	for raw, want := range map[string]string{
		"/d/logs":             "/gofile/d/logs",
		"/gofile/d/logs":      "/gofile/d/logs",
		"/gofile/../admin":    "/gofile/",
		"https://example.com": "/gofile/",
		"//example.com":       "/gofile/",
	} {
		if got := safeNextPath(raw); got != want {
			t.Fatalf("safeNextPath(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestMountedBasePathRoutesAndGeneratesPublicURLs(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath, previousTemplates := conf.GoFile, reader, uploader, basePath, templateSets
	conf.GoFile, reader, uploader, basePath = t.TempDir(), false, false, "/gofile"
	t.Cleanup(func() { conf.GoFile, reader, uploader, basePath, templateSets = previousRoot, previousReader, previousUploader, previousBasePath, previousTemplates })
	if err := os.WriteFile(conf.GoFile+string(os.PathSeparator)+"a.txt", []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	handler := withBasePath(newRouter(testManager(t)))

	request := httptest.NewRequest(http.MethodGet, "/gofile/d/a", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if location := response.Header().Get("Location"); location != "/gofile/login?next=%2Fgofile%2Fd%2Fa" {
		t.Fatalf("mounted login redirect = %q", location)
	}

	request = httptest.NewRequest(http.MethodGet, "/d/a", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if location := response.Header().Get("Location"); location != "/gofile/login?next=%2Fgofile%2Fd%2Fa" {
		t.Fatalf("stripped-prefix login redirect = %q", location)
	}

	loginBody := url.Values{"username": []string{"admin"}, "password": []string{"a durable password"}, "next": []string{"/gofile/d/a"}}.Encode()
	request = httptest.NewRequest(http.MethodPost, "/gofile/login", bytes.NewBufferString(loginBody))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if location := response.Header().Get("Location"); location != "/gofile/d/a" {
		t.Fatalf("mounted login destination = %q", location)
	}
	cookie := response.Result().Cookies()[0]
	if cookie.Path != "/gofile" {
		t.Fatalf("mounted cookie path = %q", cookie.Path)
	}

	request = httptest.NewRequest(http.MethodGet, "/gofile/", nil)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("mounted listing status = %d", response.Code)
	}
	page := response.Body.String()
	for _, want := range []string{"href=\"/gofile/\"", "href=\"/gofile/download/a.txt\"", "const appBase = \"/gofile\""} {
		if !strings.Contains(page, want) {
			t.Fatalf("mounted listing missing %q", want)
		}
	}
	if strings.Contains(page, "apiCommand") || strings.Contains(page, "copyApi") {
		t.Fatal("listing still exposes API upload command")
	}

	request = httptest.NewRequest(http.MethodGet, "/gofile/login", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !strings.Contains(response.Body.String(), "action=\"/gofile/login\"") {
		t.Fatal("mounted login form action is not prefixed")
	}

	request = httptest.NewRequest(http.MethodGet, "/gofile-other/", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("base path lookalike status = %d", response.Code)
	}
}

func TestRouterRequiresAuthentication(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousTemplates := conf.GoFile, reader, uploader, templateSets
	conf.GoFile, reader, uploader = t.TempDir(), false, false
	t.Cleanup(func() { conf.GoFile, reader, uploader, templateSets = previousRoot, previousReader, previousUploader, previousTemplates })
	if err := os.WriteFile(conf.GoFile+string(os.PathSeparator)+"a.txt", []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	router := newRouter(testManager(t))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d", response.Code)
	}
	cookie := loginCookie(t, router)
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("authenticated list = %d", response.Code)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store, private" {
		t.Fatalf("directory cache-control = %q", got)
	}
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