package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/irains/fileharbor/auth"
	"github.com/irains/fileharbor/conf"
)

func testManager(t *testing.T) *auth.Manager {
	t.Helper()
	passwordHash, err := auth.GeneratePasswordHash([]byte("a durable password"))
	if err != nil {
		t.Fatal(err)
	}
	manager, err := auth.NewManager(auth.Config{
		Username:      "admin",
		PasswordHash:  passwordHash,
		SessionSecret: "0123456789abcdef0123456789abcdef",
		APIToken:      "abcdef0123456789abcdef0123456789",
		CookiePath:    basePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func newTestState(t *testing.T) *RuntimeState {
	t.Helper()
	state, err := OpenRuntimeState(t.TempDir(), conf.FileHarbor)
	if err != nil {
		t.Fatal(err)
	}
	state.SetReady(true)
	t.Cleanup(func() {
		if err := state.Close(); err != nil {
			t.Error(err)
		}
	})
	return state
}

func newTestRouter(t *testing.T, manager *auth.Manager) *gin.Engine {
	t.Helper()
	return newRouter(manager, newTestState(t))
}

func sessionCookieFromResponse(t *testing.T, response *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	var sessionCookie, legacyCookie *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		switch cookie.Name {
		case auth.CookieName:
			sessionCookie = cookie
		case auth.LegacyCookieName:
			legacyCookie = cookie
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected FileHarbor session cookie")
	}
	if legacyCookie == nil || legacyCookie.MaxAge != -1 {
		t.Fatal("expected expired legacy session cookie")
	}
	return sessionCookie
}

func loginCookie(t *testing.T, router http.Handler) *http.Cookie {
	t.Helper()
	body := url.Values{"username": []string{"admin"}, "password": []string{"a durable password"}}.Encode()
	request := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("login status = %d", response.Code)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store, private" {
		t.Fatalf("login cache-control = %q", got)
	}
	return sessionCookieFromResponse(t, response)
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
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, "/gofile"
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})
	if err := os.WriteFile(filepath.Join(conf.FileHarbor, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	handler := withBasePath(newTestRouter(t, testManager(t)))

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
	cookie := sessionCookieFromResponse(t, response)
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
	bundle, err := loadWebAssets()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/gofile/assets/" + bundle.entry.File, "fileharbor-base\" content=\"/gofile", "fileharbor-nonce"} {
		if !strings.Contains(page, want) {
			t.Fatalf("mounted shell missing %q", want)
		}
	}

	request = httptest.NewRequest(http.MethodGet, "/gofile/assets/"+bundle.entry.File, nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("mounted asset status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/gofile/login", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !strings.Contains(response.Body.String(), "id=\"root\"") {
		t.Fatal("mounted login does not serve the SPA shell")
	}

	request = httptest.NewRequest(http.MethodGet, "/gofile-other/", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("base path lookalike status = %d", response.Code)
	}
}

func TestHealthAndReadinessRoutes(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})
	state := newTestState(t)
	router := newRouter(testManager(t), state)

	for _, target := range []string{"/healthz", "/readyz"} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s = status %d, cache %q", target, response.Code, response.Header().Get("Cache-Control"))
		}
	}

	state.SetReady(false)
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "shutting_down") {
		t.Fatalf("unready response = %d, %s", response.Code, response.Body.String())
	}
}

func TestRequestGateRejectsNewRequestsAndDrainsAdmittedHandlers(t *testing.T) {
	gate := newRequestGate()
	started := make(chan struct{})
	allowReturn := make(chan struct{})
	handler := gate.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-allowReturn
	}))
	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("admitted handler did not start")
	}

	gate.StopAdmission()
	select {
	case <-gate.Drained():
		t.Fatal("request gate drained before admitted handler returned")
	default:
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusServiceUnavailable || response.Body.String() != `{"ok":false,"code":"shutting_down"}` {
		t.Fatalf("shutdown response = %d, %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("shutdown cache-control = %q", response.Header().Get("Cache-Control"))
	}
	if gate.WaitForDrain(time.Now().Add(20 * time.Millisecond)) {
		t.Fatal("request gate reported a drain before admitted handler returned")
	}
	close(allowReturn)
	group.Wait()
	select {
	case <-gate.Drained():
	case <-time.After(time.Second):
		t.Fatal("request gate did not drain after handler returned")
	}
	if !gate.WaitForDrain(time.Now().Add(time.Second)) {
		t.Fatal("request gate did not report its completed drain")
	}
}

func TestRouterRequiresAuthentication(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})
	if err := os.WriteFile(filepath.Join(conf.FileHarbor, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	router := newTestRouter(t, testManager(t))
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
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})
	router := newTestRouter(t, testManager(t))
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

func multipartRequest(t *testing.T, target string, fields map[string]string, name string, content []byte) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if name != "" {
		part, err := writer.CreateFormFile("file", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, target, body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func multipartRequestWithFileField(t *testing.T, target, field, name string, content []byte) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(field, name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, target, body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestChunkRoutesRejectInvalidIdentifiersAndDuplicateParts(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})
	manager := testManager(t)
	router := newTestRouter(t, manager)
	cookie := loginCookie(t, router)

	request := multipartRequest(t, "/do/chunk/upload", map[string]string{
		"fileId": "invalid:id", "chunkIndex": "0", "totalChunks": "1",
	}, "part", []byte("one"))
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", sessionCSRF(t, manager, cookie))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid chunk identifier status = %d: %s", response.Code, response.Body.String())
	}

	for attempt, want := range []int{http.StatusOK, http.StatusConflict} {
		request = multipartRequest(t, "/do/chunk/upload", map[string]string{
			"fileId": "transfer_1", "chunkIndex": "0", "totalChunks": "1",
		}, "part", []byte("one"))
		request.AddCookie(cookie)
		request.Header.Set("X-CSRF-Token", sessionCSRF(t, manager, cookie))
		response = httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("chunk upload %d status = %d: %s", attempt, response.Code, response.Body.String())
		}
	}
}

func TestUploadRoutesCleanUnexpectedMultipartFiles(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})
	manager := testManager(t)
	state := newTestState(t)
	router := newRouter(manager, state)
	cookie := loginCookie(t, router)
	csrf := sessionCSRF(t, manager, cookie)
	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)

	for _, target := range []string{"/do/upload/", "/do/chunk/upload", "/api/upload"} {
		request := multipartRequestWithFileField(t, target, "unexpected", "spill.bin", bytes.Repeat([]byte("x"), 1<<20+1))
		if target == "/api/upload" {
			request.Header.Set("Authorization", "Bearer abcdef0123456789abcdef0123456789")
		} else {
			request.AddCookie(cookie)
			request.Header.Set("X-CSRF-Token", csrf)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("unexpected file field at %q = %d: %s", target, response.Code, response.Body.String())
		}
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("multipart spill files remain after %q: %v", target, entries)
		}
	}
}

func TestUploadRoutesRejectOversizedRequestBodies(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	previousUploadLimit, previousChunkLimit := maxUploadBodyBytes, maxChunkBodyBytes
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	maxUploadBodyBytes, maxChunkBodyBytes = 1024, 1024
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
		maxUploadBodyBytes, maxChunkBodyBytes = previousUploadLimit, previousChunkLimit
	})
	manager := testManager(t)
	router := newTestRouter(t, manager)
	cookie := loginCookie(t, router)
	csrf := sessionCSRF(t, manager, cookie)

	requests := []*http.Request{
		multipartRequest(t, "/do/upload/", nil, "too-large.bin", bytes.Repeat([]byte("x"), 2048)),
		multipartRequest(t, "/do/chunk/upload", map[string]string{"fileId": "transfer", "chunkIndex": "0", "totalChunks": "1"}, "part", bytes.Repeat([]byte("x"), 2048)),
	}
	for _, request := range requests {
		request.AddCookie(cookie)
		request.Header.Set("X-CSRF-Token", csrf)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("oversized upload %q = %d: %s", request.URL.Path, response.Code, response.Body.String())
		}
	}
}

func TestChunkUploadRespectsGlobalStorageQuota(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	previousStorageLimit := maxChunkStorageBytes
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	maxChunkStorageBytes = 5
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
		maxChunkStorageBytes = previousStorageLimit
	})
	manager := testManager(t)
	router := newTestRouter(t, manager)
	cookie := loginCookie(t, router)
	csrf := sessionCSRF(t, manager, cookie)

	for index, fileID := range []string{"first", "second"} {
		request := multipartRequest(t, "/do/chunk/upload", map[string]string{"fileId": fileID, "chunkIndex": "0", "totalChunks": "1"}, "part", []byte("12345"))
		request.AddCookie(cookie)
		request.Header.Set("X-CSRF-Token", csrf)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		want := http.StatusOK
		if index == 1 {
			want = http.StatusRequestEntityTooLarge
		}
		if response.Code != want {
			t.Fatalf("global quota upload %d = %d: %s", index, response.Code, response.Body.String())
		}
	}
}

func sessionCSRF(t *testing.T, manager *auth.Manager, cookie *http.Cookie) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	info, ok := manager.SessionFromRequest(request)
	if !ok {
		t.Fatal("test session cookie was not accepted")
	}
	return info.CSRF
}

func TestChunkMergeRequiresCompleteStableSet(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})
	manager := testManager(t)
	state := newTestState(t)
	router := newRouter(manager, state)
	cookie := loginCookie(t, router)
	csrf := sessionCSRF(t, manager, cookie)
	if err := os.MkdirAll(filepath.Join(state.ChunksDir, "partial"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state.ChunksDir, "partial", "0"), []byte("only one"), 0600); err != nil {
		t.Fatal(err)
	}

	request := multipartRequest(t, "/do/chunk/merge", map[string]string{
		"fileId": "partial", "totalChunks": "2", "path": "", "fileName": "merged.txt",
	}, "", nil)
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("partial merge status = %d: %s", response.Code, response.Body.String())
	}
	if _, err := os.Stat(filepath.Join(conf.FileHarbor, "merged.txt")); !os.IsNotExist(err) {
		t.Fatalf("partial merge published output: %v", err)
	}
}

func TestEditorRejectsOversizedAndLegacySaveRoutes(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})
	manager := testManager(t)
	router := newTestRouter(t, manager)
	cookie := loginCookie(t, router)
	csrf := sessionCSRF(t, manager, cookie)
	if err := os.WriteFile(filepath.Join(conf.FileHarbor, "large.txt"), bytes.Repeat([]byte("x"), int(maxEditorBytes)+1), 0644); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/edit/large.txt", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large editor status = %d: %s", response.Code, response.Body.String())
	}

	if err := os.WriteFile(filepath.Join(conf.FileHarbor, "notes.txt"), []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	body := url.Values{"path": {"notes.txt"}, "data": {"after"}}.Encode()
	request = httptest.NewRequest(http.MethodPost, "/do/save", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-CSRF-Token", csrf)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("legacy editor save status = %d: %s", response.Code, response.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(conf.FileHarbor, "notes.txt"))
	if err != nil || string(data) != "before" {
		t.Fatalf("legacy editor save modified data = %q, %v", data, err)
	}
}

func TestAPIUploadAllowsBearerOnly(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})
	router := newTestRouter(t, testManager(t))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer abcdef0123456789abcdef0123456789")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("bearer must not browse, got %d", response.Code)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("path", ""); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", "upload.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("uploaded")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/upload", body)
	request.Header.Set("Authorization", "Bearer abcdef0123456789abcdef0123456789")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("bearer upload status = %d: %s", response.Code, response.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(conf.FileHarbor, "upload.txt"))
	if err != nil || string(data) != "uploaded" {
		t.Fatalf("uploaded file = %q, %v", data, err)
	}
}

func TestLoginRejectsUnsafeNextAndPreviewIsPlainText(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})
	if err := os.WriteFile(filepath.Join(conf.FileHarbor, "payload.html"), []byte("<script>alert(1)</script>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(conf.FileHarbor, "notes.txt"), []byte("safe text"), 0644); err != nil {
		t.Fatal(err)
	}
	router := newTestRouter(t, testManager(t))
	body := url.Values{"username": []string{"admin"}, "password": []string{"a durable password"}, "next": []string{"/\\\\evil.example"}}.Encode()
	request := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if location := response.Header().Get("Location"); location != "/" {
		t.Fatalf("unsafe next redirected to %q", location)
	}
	cookie := sessionCookieFromResponse(t, response)

	request = httptest.NewRequest(http.MethodGet, "/view/payload.html", nil)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if got := response.Header().Get("Content-Disposition"); got == "" {
		t.Fatal("active content preview must download")
	}
	if got := response.Header().Get("Content-Type"); got == "text/html; charset=utf-8" {
		t.Fatalf("active content type = %q", got)
	}
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("nosniff = %q", got)
	}
	if got := response.Header().Get("Content-Security-Policy"); got != "default-src 'none'; base-uri 'none'; form-action 'none'; sandbox" {
		t.Fatalf("active content CSP = %q", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store, private" {
		t.Fatalf("cache-control = %q", got)
	}

	request = httptest.NewRequest(http.MethodGet, "/view/notes.txt", nil)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if got := response.Header().Get("Content-Security-Policy"); got != "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; sandbox" {
		t.Fatalf("text preview CSP = %q", got)
	}

	request = httptest.NewRequest(http.MethodGet, "/view/missing.txt", nil)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing preview status = %d", response.Code)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store, private" {
		t.Fatalf("missing preview cache-control = %q", got)
	}
}
