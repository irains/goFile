package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irains/fileharbor/conf"
)

type directoryBrowserResponse struct {
	OK   bool   `json:"ok"`
	Path string `json:"path"`
	Dirs []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"dirs"`
}

type apiErrorResponse struct {
	OK   bool   `json:"ok"`
	Code string `json:"code"`
}

func requestDirectoryBrowser(t *testing.T, router http.Handler, cookie *http.Cookie, target string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestDirectoryBrowserAPI(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath, previousTemplates := conf.FileHarbor, reader, uploader, basePath, templateSets
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath, templateSets = previousRoot, previousReader, previousUploader, previousBasePath, previousTemplates
	})

	for _, dir := range []string{"alpha", "empty", filepath.Join("alpha", "nested")} {
		if err := os.MkdirAll(filepath.Join(conf.FileHarbor, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(conf.FileHarbor, "visible.txt"), []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}

	router := newTestRouter(t, testManager(t))
	response := requestDirectoryBrowser(t, router, nil, "/api/directories")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous directory browser status = %d", response.Code)
	}

	cookie := loginCookie(t, router)
	response = requestDirectoryBrowser(t, router, cookie, "/api/directories")
	if response.Code != http.StatusOK {
		t.Fatalf("root directory browser status = %d", response.Code)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store, private" {
		t.Fatalf("directory browser cache-control = %q", got)
	}
	var root directoryBrowserResponse
	if err := json.Unmarshal(response.Body.Bytes(), &root); err != nil {
		t.Fatal(err)
	}
	if !root.OK || root.Path != "" || len(root.Dirs) != 2 || root.Dirs[0].Name != "alpha" || root.Dirs[0].Path != "alpha" || root.Dirs[1].Name != "empty" || root.Dirs[1].Path != "empty" {
		t.Fatalf("root directory browser response = %#v", root)
	}

	response = requestDirectoryBrowser(t, router, cookie, "/api/directories?path=alpha")
	if response.Code != http.StatusOK {
		t.Fatalf("nested directory browser status = %d", response.Code)
	}
	var nested directoryBrowserResponse
	if err := json.Unmarshal(response.Body.Bytes(), &nested); err != nil {
		t.Fatal(err)
	}
	if nested.Path != "alpha" || len(nested.Dirs) != 1 || nested.Dirs[0].Name != "nested" || nested.Dirs[0].Path != "alpha/nested" {
		t.Fatalf("nested directory browser response = %#v", nested)
	}

	for _, test := range []struct {
		target string
		code   string
	}{
		{"/api/directories?path=..", "invalid_path"},
		{"/api/directories?path=visible.txt", "not_directory"},
		{"/api/directories?path=missing", "not_found"},
	} {
		response = requestDirectoryBrowser(t, router, cookie, test.target)
		if response.Code != http.StatusBadRequest && response.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d", test.target, response.Code)
		}
		var body apiErrorResponse
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.OK || body.Code != test.code {
			t.Fatalf("%s response = %#v", test.target, body)
		}
	}
}

func TestDirectoryBrowserRemainsAvailableReadOnlyAndMounted(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath, previousTemplates := conf.FileHarbor, reader, uploader, basePath, templateSets
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), true, false, "/gofile"
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath, templateSets = previousRoot, previousReader, previousUploader, previousBasePath, previousTemplates
	})
	if err := os.Mkdir(filepath.Join(conf.FileHarbor, "reports"), 0755); err != nil {
		t.Fatal(err)
	}

	router := withBasePath(newTestRouter(t, testManager(t)))
	cookie := loginCookie(t, router)
	response := requestDirectoryBrowser(t, router, cookie, "/gofile/api/directories")
	if response.Code != http.StatusOK {
		t.Fatalf("mounted read-only directory browser status = %d", response.Code)
	}
	var body directoryBrowserResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Path != "" || len(body.Dirs) != 1 || body.Dirs[0].Path != "reports" {
		t.Fatalf("mounted directory browser response = %#v", body)
	}
}

func TestDirectoryRowsExposeRefreshAndSafeParentNavigation(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath, previousTemplates := conf.FileHarbor, reader, uploader, basePath, templateSets
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath, templateSets = previousRoot, previousReader, previousUploader, previousBasePath, previousTemplates
	})
	if err := os.MkdirAll(filepath.Join(conf.FileHarbor, "nested", "empty"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(conf.FileHarbor, "nested", "document.txt"), []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}

	router := newTestRouter(t, testManager(t))
	cookie := loginCookie(t, router)
	response := requestDirectoryBrowser(t, router, cookie, "/d/nested")
	if response.Code != http.StatusOK {
		t.Fatalf("nested listing status = %d", response.Code)
	}
	page := response.Body.String()
	for _, want := range []string{"id=\"refreshButton\"", "<tr class=\"parent-row\">", "href=\"/\"", "data-entry=\"true\"", "tbody tr[data-entry=\"true\"]"} {
		if !strings.Contains(page, want) {
			t.Fatalf("nested listing missing %q", want)
		}
	}
	parentStart := strings.Index(page, "<tr class=\"parent-row\">")
	parentEnd := strings.Index(page[parentStart:], "</tr>")
	if parentStart < 0 || parentEnd < 0 {
		t.Fatal("parent row was not rendered as a complete row")
	}
	parentRow := page[parentStart : parentStart+parentEnd]
	for _, forbidden := range []string{"data-entry", "entry-check", "data-name", "data-path", "dropdown"} {
		if strings.Contains(parentRow, forbidden) {
			t.Fatalf("parent row unexpectedly contains %q: %s", forbidden, parentRow)
		}
	}

	response = requestDirectoryBrowser(t, router, cookie, "/d/nested/empty")
	if response.Code != http.StatusOK {
		t.Fatalf("empty nested listing status = %d", response.Code)
	}
	page = response.Body.String()
	if !strings.Contains(page, "<tr class=\"parent-row\">") || strings.Contains(page, "<section class=\"empty-state\"") || !strings.Contains(page, "href=\"/d/nested\"") {
		t.Fatalf("empty nested listing did not retain parent navigation: %s", page)
	}
}

func TestReliableUploadQueueRendersAcrossUploadModes(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath, previousTemplates, previousUploads := conf.FileHarbor, reader, uploader, basePath, templateSets, uploads
	conf.FileHarbor, reader, uploader, basePath, uploads = t.TempDir(), false, false, "/fileharbor", nil
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath, templateSets, uploads = previousRoot, previousReader, previousUploader, previousBasePath, previousTemplates, previousUploads
	})

	renderPage := func(t *testing.T, writable, uploadOnly bool) string {
		t.Helper()
		reader, uploader, uploads = !writable, uploadOnly, nil
		handler := withBasePath(newTestRouter(t, testManager(t)))
		cookie := loginCookie(t, handler)
		response := requestDirectoryBrowser(t, handler, cookie, "/fileharbor/")
		if response.Code != http.StatusOK {
			t.Fatalf("listing status = %d: %s", response.Code, response.Body.String())
		}
		return response.Body.String()
	}

	for _, mode := range []struct {
		name       string
		writable   bool
		uploadOnly bool
		visible    bool
	}{
		{name: "normal", writable: true, visible: true},
		{name: "upload-only", uploadOnly: true, visible: true},
		{name: "strict-read-only", visible: false},
	} {
		t.Run(mode.name, func(t *testing.T) {
			page := renderPage(t, mode.writable, mode.uploadOnly)
			if mode.visible {
				for _, want := range []string{"id=\"fileInput\" multiple", "id=\"uploadQueue\" role=\"list\"", "appURL('/api/uploads", "XMLHttpRequest", "indexedDB.open"} {
					if !strings.Contains(page, want) {
						t.Fatalf("reliable upload page missing %q", want)
					}
				}
				for _, forbidden := range []string{"id=\"uploadProgress\"", "id=\"uploadProgressBar\"", "'/do/upload/'", "new FormData()"} {
					if strings.Contains(page, forbidden) {
						t.Fatalf("reliable upload page retained legacy %q", forbidden)
					}
				}
			} else if strings.Contains(page, "id=\"fileInput\"") || strings.Contains(page, "id=\"uploadQueue\"") {
				t.Fatal("strict read-only page rendered upload queue")
			}
		})
	}
}

func TestRootEmptyListingRemainsAnEmptyState(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath, previousTemplates := conf.FileHarbor, reader, uploader, basePath, templateSets
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath, templateSets = previousRoot, previousReader, previousUploader, previousBasePath, previousTemplates
	})

	router := newTestRouter(t, testManager(t))
	cookie := loginCookie(t, router)
	response := requestDirectoryBrowser(t, router, cookie, "/")
	if response.Code != http.StatusOK {
		t.Fatalf("empty root listing status = %d", response.Code)
	}
	page := response.Body.String()
	if !strings.Contains(page, "<section class=\"empty-state\"") || strings.Contains(page, "<tr class=\"parent-row\">") {
		t.Fatalf("root empty listing state = %s", page)
	}
}
