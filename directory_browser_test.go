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
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
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
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), true, false, "/gofile"
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
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

func TestDirectoryListingShellAndAPI(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
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
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "id=\"root\"") {
		t.Fatalf("nested shell response = %d: %s", response.Code, response.Body.String())
	}

	response = requestDirectoryBrowser(t, router, cookie, "/api/listing?path=nested")
	if response.Code != http.StatusOK {
		t.Fatalf("nested listing API status = %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		OK        bool `json:"ok"`
		Directory struct {
			Path         string  `json:"path"`
			ParentPath   *string `json:"parent_path"`
			ListingToken string  `json:"listing_token"`
			Entries      []struct {
				Name string `json:"name"`
				Path string `json:"path"`
			} `json:"entries"`
		} `json:"directory"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || body.Directory.Path != "nested" || body.Directory.ParentPath == nil || *body.Directory.ParentPath != "" || body.Directory.ListingToken == "" || len(body.Directory.Entries) != 2 || body.Directory.Entries[0].Path != "nested/empty" || body.Directory.Entries[1].Name != "document.txt" {
		t.Fatalf("nested listing response = %#v", body)
	}
}

func TestListingCapabilitiesReflectUploadModes(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath, previousUploads := conf.FileHarbor, reader, uploader, basePath, uploads
	conf.FileHarbor, reader, uploader, basePath, uploads = t.TempDir(), false, false, "/fileharbor", nil
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath, uploads = previousRoot, previousReader, previousUploader, previousBasePath, previousUploads
	})

	for _, mode := range []struct {
		name       string
		writable   bool
		uploadOnly bool
		upload     bool
		editor     bool
	}{
		{name: "normal", writable: true, upload: true, editor: true},
		{name: "upload-only", uploadOnly: true, upload: true},
		{name: "strict-read-only"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			reader, uploader, uploads = !mode.writable, mode.uploadOnly, nil
			handler := withBasePath(newTestRouter(t, testManager(t)))
			cookie := loginCookie(t, handler)
			response := requestDirectoryBrowser(t, handler, cookie, "/fileharbor/api/session")
			if response.Code != http.StatusOK {
				t.Fatalf("listing status = %d: %s", response.Code, response.Body.String())
			}
			var body struct {
				Capabilities struct {
					Upload     bool `json:"upload"`
					EditorSave bool `json:"editor_save"`
				} `json:"capabilities"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Capabilities.Upload != mode.upload || body.Capabilities.EditorSave != mode.editor {
				t.Fatalf("capabilities = %#v", body.Capabilities)
			}
		})
	}
}

func TestRootEmptyListingReturnsEmptyEntries(t *testing.T) {
	previousRoot, previousReader, previousUploader, previousBasePath := conf.FileHarbor, reader, uploader, basePath
	conf.FileHarbor, reader, uploader, basePath = t.TempDir(), false, false, ""
	t.Cleanup(func() {
		conf.FileHarbor, reader, uploader, basePath = previousRoot, previousReader, previousUploader, previousBasePath
	})

	router := newTestRouter(t, testManager(t))
	cookie := loginCookie(t, router)
	response := requestDirectoryBrowser(t, router, cookie, "/api/listing")
	if response.Code != http.StatusOK {
		t.Fatalf("empty root listing status = %d", response.Code)
	}
	var body struct {
		Directory struct {
			Entries []json.RawMessage `json:"entries"`
		} `json:"directory"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Directory.Entries == nil || len(body.Directory.Entries) != 0 {
		t.Fatalf("empty root entries = %#v", body.Directory.Entries)
	}
}
