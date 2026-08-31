package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irains/fileharbor/conf"
)

func TestReliableUploadRoutesUseSessionCSRFAndStableErrors(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath, previousUploads := conf.FileHarbor, reader, uploader, basePath, uploads
	conf.FileHarbor, reader, uploader, basePath, uploads = t.TempDir(), false, false, "", nil
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath, uploads = previousRoot, previousReader, previousUploader, previousBasePath, previousUploads
	})
	manager := testManager(t)
	router := newTestRouter(t, manager)
	cookie := loginCookie(t, router)
	csrf := sessionCSRF(t, manager, cookie)
	contents := []byte("handler reliable upload")
	digest := digestForHandlerTest(contents)

	request := uploadCreateRequest("", "handler.txt", int64(len(contents)), testUploadID, testUploadToken)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusForbidden, "csrf_invalid")

	request = uploadCreateRequest("", "handler.txt", int64(len(contents)), testUploadID, testUploadToken)
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusCreated, "")

	request = httptest.NewRequest(http.MethodPut, "/api/uploads/"+testUploadID+"/parts/0", bytes.NewReader(contents))
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	request.Header.Set(uploadTokenHeader, testUploadToken)
	request.Header.Set(uploadPartDigestHeader, digest)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusOK, "")

	request = httptest.NewRequest(http.MethodPost, "/api/uploads/"+testUploadID+"/complete", nil)
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	request.Header.Set(uploadTokenHeader, testUploadToken)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusOK, "")
	data, err := os.ReadFile(filepath.Join(conf.FileHarbor, "handler.txt"))
	if err != nil || !bytes.Equal(data, contents) {
		t.Fatalf("completed handler upload = %q, %v", data, err)
	}
}

func TestReliableUploadRoutesAllowScopedBearerAndMapErrors(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath, previousUploads := conf.FileHarbor, reader, uploader, basePath, uploads
	conf.FileHarbor, reader, uploader, basePath, uploads = t.TempDir(), false, false, "", nil
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath, uploads = previousRoot, previousReader, previousUploader, previousBasePath, previousUploads
	})
	router := newTestRouter(t, testManager(t))
	contents := []byte("bearer route")
	digest := digestForHandlerTest(contents)

	request := uploadCreateRequest("", "taken.txt", int64(len(contents)), testUploadID, testUploadToken)
	request.Header.Set("Authorization", "Bearer abcdef0123456789abcdef0123456789")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusCreated, "")

	request = httptest.NewRequest(http.MethodPut, "/api/uploads/"+testUploadID+"/parts/0", bytes.NewReader(contents))
	request.Header.Set("Authorization", "Bearer abcdef0123456789abcdef0123456789")
	request.Header.Set(uploadTokenHeader, testUploadToken)
	request.Header.Set(uploadPartDigestHeader, digest)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusOK, "")

	if err := os.WriteFile(filepath.Join(conf.FileHarbor, "taken.txt"), []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/uploads/"+testUploadID+"/complete", nil)
	request.Header.Set("Authorization", "Bearer abcdef0123456789abcdef0123456789")
	request.Header.Set(uploadTokenHeader, testUploadToken)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusConflict, "destination_exists")

	request = httptest.NewRequest(http.MethodGet, "/api/uploads/"+testUploadID, nil)
	request.Header.Set("Authorization", "Bearer abcdef0123456789abcdef0123456789")
	request.Header.Set(uploadTokenHeader, strings.Repeat("f", 64))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusNotFound, "upload_not_found")
}

func TestReliableUploadRoutesWorkBelowBasePath(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath, previousUploads := conf.FileHarbor, reader, uploader, basePath, uploads
	conf.FileHarbor, reader, uploader, basePath, uploads = t.TempDir(), false, false, "/fileharbor", nil
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath, uploads = previousRoot, previousReader, previousUploader, previousBasePath, previousUploads
	})
	handler := withBasePath(newTestRouter(t, testManager(t)))
	request := uploadCreateRequest("", "mounted.txt", 0, testUploadID, testUploadToken)
	request.URL.Path = "/fileharbor/api/uploads"
	request.URL.RawPath = ""
	request.RequestURI = request.URL.RequestURI()
	request.Header.Set("Authorization", "Bearer abcdef0123456789abcdef0123456789")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusCreated, "")
}

func TestReliableUploadRoutesRespectReadOnlyModes(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath, previousUploads := conf.FileHarbor, reader, uploader, basePath, uploads
	conf.FileHarbor, reader, uploader, basePath, uploads = t.TempDir(), true, false, "", nil
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath, uploads = previousRoot, previousReader, previousUploader, previousBasePath, previousUploads
	})
	manager := testManager(t)
	state := newTestState(t)
	readOnlyRouter := newRouter(manager, state)
	request := uploadCreateRequest("", "blocked.txt", 0, testUploadID, testUploadToken)
	request.Header.Set("Authorization", "Bearer abcdef0123456789abcdef0123456789")
	response := httptest.NewRecorder()
	readOnlyRouter.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("read-only reliable upload status = %d: %s", response.Code, response.Body.String())
	}

	uploader, uploads = true, nil
	uploadOnlyRouter := newRouter(manager, newTestState(t))
	request = uploadCreateRequest("", "allowed.txt", 0, testUploadIDTwo, testUploadToken)
	request.Header.Set("Authorization", "Bearer abcdef0123456789abcdef0123456789")
	response = httptest.NewRecorder()
	uploadOnlyRouter.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusCreated, "")
}

func TestReliableUploadFinalPartLimitMapsToPayloadTooLarge(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath, previousUploads := conf.FileHarbor, reader, uploader, basePath, uploads
	conf.FileHarbor, reader, uploader, basePath, uploads = t.TempDir(), false, false, "", nil
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath, uploads = previousRoot, previousReader, previousUploader, previousBasePath, previousUploads
	})
	manager := testManager(t)
	router := newTestRouter(t, manager)
	cookie := loginCookie(t, router)
	csrf := sessionCSRF(t, manager, cookie)
	if uploads == nil {
		t.Fatal("test router did not initialize uploads")
	}
	contents := bytes.Repeat([]byte("x"), int(uploads.config.ChunkBytes)+1)
	request := uploadCreateRequest("", "final-part.bin", int64(len(contents)), testUploadID, testUploadToken)
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusCreated, "")

	finalPart := bytes.Repeat([]byte("x"), 2)
	request = httptest.NewRequest(http.MethodPut, "/api/uploads/"+testUploadID+"/parts/1", bytes.NewReader(finalPart))
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	request.Header.Set(uploadTokenHeader, testUploadToken)
	request.Header.Set(uploadPartDigestHeader, digestForHandlerTest(finalPart))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusRequestEntityTooLarge, "size_mismatch")
}

func TestReliableUploadPartLimitMapsToPayloadTooLarge(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath, previousUploads := conf.FileHarbor, reader, uploader, basePath, uploads
	conf.FileHarbor, reader, uploader, basePath, uploads = t.TempDir(), false, false, "", nil
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath, uploads = previousRoot, previousReader, previousUploader, previousBasePath, previousUploads
	})
	manager := testManager(t)
	router := newTestRouter(t, manager)
	cookie := loginCookie(t, router)
	csrf := sessionCSRF(t, manager, cookie)
	if uploads == nil {
		t.Fatal("test router did not initialize uploads")
	}
	originalChunkBytes := uploads.config.ChunkBytes
	uploads.config.ChunkBytes = 1 << 20
	t.Cleanup(func() { uploads.config.ChunkBytes = originalChunkBytes })

	body := bytes.Repeat([]byte("x"), int(uploads.config.ChunkBytes)+1)
	request := uploadCreateRequest("", "oversized.bin", int64(len(body)), testUploadID, testUploadToken)
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusCreated, "")

	request = httptest.NewRequest(http.MethodPut, "/api/uploads/"+testUploadID+"/parts/0", bytes.NewReader(body))
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	request.Header.Set(uploadTokenHeader, testUploadToken)
	request.Header.Set(uploadPartDigestHeader, digestForHandlerTest(body))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertUploadHTTPCode(t, response, http.StatusRequestEntityTooLarge, "size_mismatch")
}

func uploadCreateRequest(path, name string, size int64, id, token string) *http.Request {
	body, _ := json.Marshal(UploadCreateRequest{Path: path, Name: name, Size: size})
	request := httptest.NewRequest(http.MethodPost, "/api/uploads", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(uploadIDHeader, id)
	request.Header.Set(uploadTokenHeader, token)
	return request
}

func assertUploadHTTPCode(t *testing.T, response *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d: %s", response.Code, wantStatus, response.Body.String())
	}
	if wantCode == "" {
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Code != wantCode {
		t.Fatalf("response code = %q, err=%v, want %q: %s", body.Code, err, wantCode, response.Body.String())
	}
}

func digestForHandlerTest(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}
