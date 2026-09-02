package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/irains/fileharbor/assets"
)

const webRoot = "web"

// webManifest is the small subset of Vite's manifest used to locate the entry
// module and its transitive CSS files. The server deliberately refuses unknown
// and malformed paths instead of treating the embedded filesystem as public.
type webManifest map[string]webManifestEntry

type webManifestEntry struct {
	File           string   `json:"file"`
	Name           string   `json:"name"`
	Src            string   `json:"src"`
	CSS            []string `json:"css"`
	Assets         []string `json:"assets"`
	Imports        []string `json:"imports"`
	DynamicImports []string `json:"dynamicImports"`
	IsEntry        bool     `json:"isEntry"`
	IsDynamicEntry bool     `json:"isDynamicEntry"`
}

type webAssets struct {
	files    fs.FS
	manifest webManifest
	entry    webManifestEntry
	public   map[string]struct{}
}

var (
	webAssetsOnce sync.Once
	loadedWeb     *webAssets
	webAssetsErr  error
)

func loadWebAssets() (*webAssets, error) {
	webAssetsOnce.Do(func() {
		root, err := fs.Sub(assets.Web, webRoot)
		if err != nil {
			webAssetsErr = fmt.Errorf("embedded web assets unavailable: %w", err)
			return
		}
		contents, err := fs.ReadFile(root, "manifest.json")
		if err != nil {
			webAssetsErr = fmt.Errorf("embedded web manifest unavailable: %w", err)
			return
		}
		var manifest webManifest
		decoder := json.NewDecoder(strings.NewReader(string(contents)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&manifest); err != nil {
			webAssetsErr = fmt.Errorf("invalid embedded web manifest: %w", err)
			return
		}
		if err := ensureManifestEOF(decoder); err != nil {
			webAssetsErr = errors.New("invalid embedded web manifest")
			return
		}

		entry, ok := manifest["index.html"]
		if !ok || !entry.IsEntry || !safeWebAssetPath(entry.File) {
			webAssetsErr = errors.New("embedded web manifest has no safe index entry")
			return
		}

		public := make(map[string]struct{})
		entryCount := 0
		for name, value := range manifest {
			if name == "" || !safeManifestName(name) || !safeWebAssetPath(value.File) {
				webAssetsErr = errors.New("embedded web manifest contains an unsafe entry")
				return
			}
			if value.IsEntry {
				entryCount++
			}
			public[value.File] = struct{}{}
			for _, stylesheet := range value.CSS {
				if !safeWebAssetPath(stylesheet) {
					webAssetsErr = errors.New("embedded web manifest contains an unsafe stylesheet")
					return
				}
				public[stylesheet] = struct{}{}
			}
			for _, asset := range value.Assets {
				if !safeWebAssetPath(asset) {
					webAssetsErr = errors.New("embedded web manifest contains an unsafe asset")
					return
				}
				public[asset] = struct{}{}
			}
			for _, imported := range append(append([]string{}, value.Imports...), value.DynamicImports...) {
				if !safeManifestName(imported) {
					webAssetsErr = errors.New("embedded web manifest contains an unsafe import")
					return
				}
				if _, ok := manifest[imported]; !ok {
					webAssetsErr = errors.New("embedded web manifest references an unknown import")
					return
				}
			}
		}
		if entryCount != 1 {
			webAssetsErr = errors.New("embedded web manifest has ambiguous entries")
			return
		}
		if err := verifyManifestFiles(root, manifest); err != nil {
			webAssetsErr = err
			return
		}
		loadedWeb = &webAssets{files: root, manifest: manifest, entry: entry, public: public}
	})
	return loadedWeb, webAssetsErr
}

func ensureManifestEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("unexpected JSON value")
	}
	return nil
}

func safeManifestName(value string) bool {
	return value == "index.html" || safeWebAssetPath(value)
}

func safeWebAssetPath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") || strings.ContainsRune(value, 0) {
		return false
	}
	cleaned := path.Clean(value)
	return cleaned == value && cleaned != "." && !strings.HasPrefix(cleaned, "../") && !strings.Contains(value, "//")
}

func verifyManifestFiles(files fs.FS, manifest webManifest) error {
	seen := make(map[string]struct{})
	check := func(name string) error {
		if _, ok := seen[name]; ok {
			return nil
		}
		info, err := fs.Stat(files, name)
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("embedded web manifest references a missing asset")
		}
		seen[name] = struct{}{}
		return nil
	}
	for _, entry := range manifest {
		if err := check(entry.File); err != nil {
			return err
		}
		for _, stylesheet := range entry.CSS {
			if err := check(stylesheet); err != nil {
				return err
			}
		}
		for _, asset := range entry.Assets {
			if err := check(asset); err != nil {
				return err
			}
		}
	}
	return nil
}

func nonce() (string, error) {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(value), nil
}

func webAssetURL(asset string) string {
	return appPath("/assets/" + asset)
}

func resolveEntryAssets(bundle *webAssets) ([]string, []string) {
	seen := make(map[string]bool)
	var styles []string
	var add func(webManifestEntry)
	add = func(entry webManifestEntry) {
		for _, imported := range entry.Imports {
			if seen[imported] {
				continue
			}
			seen[imported] = true
			child, ok := bundle.manifest[imported]
			if !ok {
				continue
			}
			add(child)
		}
		styles = append(styles, entry.CSS...)
	}
	add(bundle.entry)
	return uniqueAssetPaths(styles), []string{bundle.entry.File}
}

func uniqueAssetPaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	unique := make([]string, 0, len(paths))
	for _, asset := range paths {
		if !seen[asset] {
			seen[asset] = true
			unique = append(unique, asset)
		}
	}
	return unique
}

func spaCSP(value string) string {
	return "default-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self' 'nonce-" + value + "'; style-src 'self' 'nonce-" + value + "'; img-src 'self' data:; connect-src 'self'; font-src 'self' data:"
}

func themePrepaint(value string) string {
	nonce := htmlAttribute(value)
	return `<script nonce="` + nonce + `">(function(){var mode="system";var resolved="light";try{var raw=localStorage.getItem("fileharbor-mode");if(raw==="light"||raw==="dark"||raw==="system"){mode=raw}}catch(_){ }if(mode==="light"){resolved="light"}else if(mode==="dark"){resolved="dark"}else{resolved=(window.matchMedia&&window.matchMedia("(prefers-color-scheme: dark)").matches)?"dark":"light"}var root=document.documentElement;root.setAttribute("data-mui-color-scheme",resolved);root.style.colorScheme=resolved;}());</script><style nonce="` + nonce + `">:root{color-scheme:dark;background:#17171c;color:#eeeef3}:root[data-mui-color-scheme="light"]{color-scheme:light;background:#f8f7fb;color:#211f2a}</style>`
}

func serveShell(c *gin.Context, bundle *webAssets) {
	value, err := nonce()
	if err != nil {
		setPrivateResponse(c)
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "code": "io_error"})
		return
	}
	styles, scripts := resolveEntryAssets(bundle)
	locale := localeForRequest(c)
	loginNext := appPath("/")
	if c.Request.URL.Path == "/login" {
		loginNext = safeNextPath(c.Query("next"))
	}
	setPrivateResponse(c)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Content-Security-Policy", spaCSP(value))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "same-origin")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Cross-Origin-Resource-Policy", "same-origin")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(c.Writer, "<!doctype html><html lang=\""+htmlAttribute(locale)+"\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><meta name=\"fileharbor-base\" content=\""+htmlAttribute(basePath)+"\"><meta name=\"fileharbor-nonce\" content=\""+htmlAttribute(value)+"\"><meta name=\"fileharbor-locale\" content=\""+htmlAttribute(locale)+"\"><meta name=\"fileharbor-login-next\" content=\""+htmlAttribute(loginNext)+"\"><title>FileHarbor</title>"+themePrepaint(value))
	for _, style := range styles {
		_, _ = fmt.Fprint(c.Writer, "<link rel=\"stylesheet\" href=\""+htmlAttribute(webAssetURL(style))+"\">")
	}
	_, _ = fmt.Fprint(c.Writer, "</head><body><div id=\"root\"></div>")
	for _, script := range scripts {
		_, _ = fmt.Fprint(c.Writer, "<script type=\"module\" nonce=\""+htmlAttribute(value)+"\" src=\""+htmlAttribute(webAssetURL(script))+"\"></script>")
	}
	_, _ = fmt.Fprint(c.Writer, "</body></html>")
}

func htmlAttribute(value string) string {
	return strings.NewReplacer("&", "&amp;", "\"", "&quot;", "'", "&#39;", "<", "&lt;", ">", "&gt;").Replace(value)
}

func serveWebAsset(c *gin.Context, bundle *webAssets) {
	asset := strings.TrimPrefix(c.Param("asset"), "/")
	if asset == "manifest.json" || asset == "index.html" || !safeWebAssetPath(asset) {
		hiddenNotFound(c)
		return
	}
	if _, ok := bundle.public[asset]; !ok {
		hiddenNotFound(c)
		return
	}
	contents, err := fs.ReadFile(bundle.files, asset)
	if err != nil {
		hiddenNotFound(c)
		return
	}
	contentType := mime.TypeByExtension(filepath.Ext(asset))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cross-Origin-Resource-Policy", "same-origin")
	if isFingerprintedWebAsset(asset) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "no-store")
	}
	c.Data(http.StatusOK, contentType, contents)
}

func isFingerprintedWebAsset(asset string) bool {
	if !strings.HasPrefix(asset, "assets/") {
		return false
	}
	name := filepath.Base(asset)
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	for separator := strings.IndexByte(stem, '-'); separator != -1; {
		suffix := stem[separator+1:]
		if len(suffix) >= 8 && safeViteHash(suffix) {
			return true
		}
		next := strings.IndexByte(suffix, '-')
		if next == -1 {
			break
		}
		separator += next + 1
	}
	return false
}

func safeViteHash(value string) bool {
	for _, character := range value {
		if !(character >= '0' && character <= '9') &&
			!(character >= 'a' && character <= 'z') &&
			!(character >= 'A' && character <= 'Z') &&
			character != '_' && character != '-' {
			return false
		}
	}
	return true
}
