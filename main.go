package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"goFile/assets"
	"goFile/auth"
	"goFile/conf"
	"goFile/i18n"
	"goFile/utils"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
)

var (
	reader           bool
	uploader         bool
	cookieSecure     bool
	allowInsecureLAN bool
	basePath         string
	templateSets     map[i18n.LangType]*template.Template
)

const contextAuthInfo = "authInfo"

func initTemplates() {
	templateSets = make(map[i18n.LangType]*template.Template)
	for _, lang := range []i18n.LangType{i18n.EN, i18n.ZH} {
		l := lang
		templateSets[l] = template.Must(template.New("").Funcs(template.FuncMap{
			"t":           func(key string) string { return i18n.Translate(key, l) },
			"previewable": isInlinePreviewable,
			"appPath":     appPath,
			"js": func(value interface{}) template.JS {
				encoded, err := json.Marshal(value)
				if err != nil {
					return template.JS("null")
				}
				return template.JS(string(encoded))
			},
		}).ParseFS(assets.Templates, "templates/*"))
	}
}

func getLang(c *gin.Context) i18n.LangType {
	if lang, ok := c.Get("lang"); ok {
		return lang.(i18n.LangType)
	}
	return i18n.ZH
}

func LangMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := c.GetHeader("Accept-Language")
		langType := i18n.ZH
		if strings.Contains(strings.ToLower(lang), "en") && !strings.Contains(strings.ToLower(lang), "zh") {
			langType = i18n.EN
		}
		c.Set("lang", langType)
		c.Next()
	}
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func authInfo(c *gin.Context) auth.Info {
	if value, ok := c.Get(contextAuthInfo); ok {
		return value.(auth.Info)
	}
	return auth.Info{}
}

func renderHTML(c *gin.Context, name string, data gin.H) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	setPrivateResponse(c)
	lang := getLang(c)
	data["htmlLang"] = map[i18n.LangType]string{i18n.ZH: "zh-CN", i18n.EN: "en"}[lang]
	data["basePath"] = basePath
	data["allowUpload"] = !reader || uploader
	data["reader"] = reader
	info := authInfo(c)
	data["csrf"] = info.CSRF
	data["username"] = info.Username
	if total, free := utils.DiskUsage(conf.GoFile); total > 0 {
		used := total - free
		data["diskPct"] = int(used * 100 / total)
		data["diskFree"] = formatBytes(free)
		data["diskTotal"] = formatBytes(total)
	}
	if err := templateSets[lang].ExecuteTemplate(c.Writer, name, data); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
	}
}

func logAction(c *gin.Context, typ, path string) {
	fmt.Printf("%s  [%s]  %s  %s\n", time.Now().Format("2006-01-02 15:04:05"), typ, c.ClientIP(), path)
}

func chunkDir(fileID string) string {
	if fileID == "" || len(fileID) > 128 || strings.ContainsAny(fileID, "/\\") {
		return ""
	}
	base := filepath.Join(os.TempDir(), "goFile-chunks")
	dir := filepath.Clean(filepath.Join(base, fileID))
	if !strings.HasPrefix(dir, base+string(filepath.Separator)) {
		return ""
	}
	return dir
}

func wantsJSON(c *gin.Context) bool {
	path := c.Request.URL.Path
	return strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/do/") || c.GetHeader("Accept") == "application/json" || strings.Contains(c.GetHeader("Content-Type"), "application/json")
}

func setPrivateResponse(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	c.Header("Pragma", "no-cache")
	c.Header("Vary", "Cookie")
}

func appPath(internalPath string) string {
	if internalPath == "" || internalPath == "/" {
		if basePath == "" {
			return "/"
		}
		return basePath + "/"
	}
	return basePath + internalPath
}

func isSafeAppPath(path string) bool {
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.Contains(path, "\\") || strings.IndexFunc(path, unicode.IsControl) >= 0 {
		return false
	}
	for _, segment := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if segment == "." || segment == ".." || segment == "" && path != "/" {
			return false
		}
	}
	return true
}

func safeNextPath(raw string) string {
	decoded, err := url.PathUnescape(raw)
	if err != nil || decoded == "" || !strings.HasPrefix(decoded, "/") || strings.HasPrefix(decoded, "//") || strings.Contains(decoded, "\\") || strings.IndexFunc(decoded, unicode.IsControl) >= 0 {
		return appPath("/")
	}
	parsed, err := url.ParseRequestURI(decoded)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.Scheme != "" || strings.HasPrefix(parsed.Path, "//") || strings.Contains(parsed.Path, "\\") {
		return appPath("/")
	}
	internalPath := parsed.Path
	if basePath != "" {
		switch {
		case internalPath == basePath:
			internalPath = "/"
		case strings.HasPrefix(internalPath, basePath+"/"):
			internalPath = strings.TrimPrefix(internalPath, basePath)
		}
	}
	if !isSafeAppPath(internalPath) {
		return appPath("/")
	}
	return appPath(internalPath) + func() string {
		if parsed.RawQuery == "" {
			return ""
		}
		return "?" + parsed.RawQuery
	}()
}

func withBasePath(next http.Handler) http.Handler {
	if basePath == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != basePath && !strings.HasPrefix(r.URL.Path, basePath+"/") {
			next.ServeHTTP(w, r)
			return
		}
		request := new(http.Request)
		*request = *r
		request.URL = new(url.URL)
		*request.URL = *r.URL
		request.URL.Path = strings.TrimPrefix(r.URL.Path, basePath)
		if request.URL.Path == "" {
			request.URL.Path = "/"
		}
		if r.URL.RawPath != "" {
			request.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, basePath)
			if request.URL.RawPath == "" {
				request.URL.RawPath = "/"
			}
		}
		request.RequestURI = request.URL.RequestURI()
		next.ServeHTTP(w, request)
	})
}

func hiddenNotFound(c *gin.Context) {
	setPrivateResponse(c)
	c.Status(http.StatusNotFound)
}

func authRequired(manager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Bearer credentials deliberately authorize only the documented script
		// upload endpoint. Browser and filesystem-management routes require a
		// session cookie, so a leaked automation token cannot administer files.
		if c.Request.URL.Path == "/api/upload" && manager.IsBearerToken(c.GetHeader("Authorization")) {
			c.Set(contextAuthInfo, auth.Info{Bearer: true})
			c.Next()
			return
		}
		if info, ok := manager.SessionFromRequest(c.Request); ok {
			c.Set(contextAuthInfo, info)
			c.Next()
			return
		}
		setPrivateResponse(c)
		if wantsJSON(c) {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "code": "unauthenticated"})
		} else {
			c.Redirect(http.StatusFound, appPath("/login")+"?next="+url.QueryEscape(safeNextPath(c.Request.URL.RequestURI())))
		}
		c.Abort()
	}
}

func csrfRequired(manager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		info := authInfo(c)
		if !manager.CheckCSRF(info, c.GetHeader("X-CSRF-Token")) {
			c.JSON(http.StatusForbidden, gin.H{"ok": false, "code": "csrf_invalid"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func jsonError(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{"ok": false, "code": utils.ErrorCode(err)})
}

func operationStatus(err error) int {
	switch utils.ErrorCode(err) {
	case "destination_exists", "source_changed", "self_descendant", "destination_same_directory", "cross_device_move":
		return http.StatusConflict
	case "not_found", "not_directory":
		return http.StatusNotFound
	case "batch_limit_exceeded", "archive_limit_exceeded":
		return http.StatusRequestEntityTooLarge
	case "io_error", "execution_partial":
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}

func pathFromParam(raw string) string {
	return strings.ReplaceAll(strings.TrimPrefix(raw, "/"), "\\", "/")
}

func setSafeFilePreviewHeaders(c *gin.Context) {
	// Managed content can originate from the limited Bearer upload API. Text previews
	// are deliberately rendered as plain text in a sandboxed document, never as
	// browser-interpreted active content in this application's authenticated origin.
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; sandbox")
}

func isInlinePreviewable(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".txt", ".log", ".md", ".json", ".yaml", ".yml", ".toml", ".ini", ".conf", ".cfg", ".csv", ".go", ".py", ".java", ".c", ".h", ".cpp", ".hpp", ".sh", ".ps1":
		return true
	default:
		return false
	}
}

func renderDirectory(c *gin.Context, manager *auth.Manager, rawPath string) {
	cleanPath, err := utils.CleanRelative(rawPath, true)
	if err != nil {
		hiddenNotFound(c)
		return
	}
	info, err := utils.ListDirectory(cleanPath)
	if err != nil {
		hiddenNotFound(c)
		return
	}
	listingItems := make([]auth.ListingItem, 0, len(info.Entries))
	for _, entry := range info.Entries {
		listingItems = append(listingItems, auth.ListingItem{Name: entry.Name, Version: entry.Version})
	}
	listingToken := ""
	if session := authInfo(c); session.SessionID != "" {
		listingToken, _ = manager.IssueListing(session.SessionID, cleanPath, listingItems)
	}
	pagePath := ""
	if cleanPath != "" {
		pagePath = cleanPath + "/"
	}
	logAction(c, "浏览", pagePath)
	renderHTML(c, "index.tmpl", gin.H{
		"info":         info,
		"path":         pagePath,
		"rawPath":      cleanPath,
		"prev":         appPath(utils.GetPrevPath(cleanPath)),
		"listingToken": listingToken,
	})
}

func fileForRead(raw string) (string, string, os.FileInfo, error) {
	absolute, rel, info, err := utils.ResolveExisting(pathFromParam(raw), false)
	if err != nil {
		return "", "", nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", "", nil, utils.ErrUnsupportedType
	}
	return absolute, rel, info, nil
}

func writeUploadedFile(destination string, uploaded io.Reader) error {
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return utils.ErrDestinationExists
		}
		return errors.New("io")
	}
	_, copyErr := io.Copy(file, uploaded)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(destination)
		return errors.New("io")
	}
	return nil
}

func uploadDestination(parent, name string) (string, string, error) {
	if err := utils.ValidateLeafName(name); err != nil {
		return "", "", err
	}
	dir, rel, _, err := utils.ResolveDirectory(parent, true)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, name), filepath.ToSlash(filepath.Join(rel, name)), nil
}

type batchRequest struct {
	ListingToken string              `json:"listing_token"`
	Entries      []utils.ItemRequest `json:"entries"`
	Destination  string              `json:"destination"`
}

func decodeBatch(c *gin.Context, manager *auth.Manager) (utils.Selection, batchRequest, error) {
	var request batchRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if err := json.NewDecoder(c.Request.Body).Decode(&request); err != nil {
		return utils.Selection{}, request, utils.ErrInvalidPath
	}
	session := authInfo(c)
	if session.SessionID == "" {
		return utils.Selection{}, request, utils.ErrInvalidPath
	}
	directory, allowed, ok := manager.ReadListing(session.SessionID, request.ListingToken)
	if !ok {
		return utils.Selection{}, request, utils.ErrSourceChanged
	}
	selection, err := utils.ValidateSelection(directory, allowed, request.Entries)
	return selection, request, err
}

func newRouter(manager *auth.Manager) *gin.Engine {
	initTemplates()
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.Use(gin.Recovery(), LangMiddleware())
	router.GET("/login", func(c *gin.Context) {
		setPrivateResponse(c)
		if _, ok := manager.SessionFromRequest(c.Request); ok {
			c.Redirect(http.StatusFound, appPath("/"))
			return
		}
		renderHTML(c, "login.tmpl", gin.H{"next": safeNextPath(c.Query("next"))})
	})
	router.POST("/login", func(c *gin.Context) {
		setPrivateResponse(c)
		username := c.PostForm("username")
		password := c.PostForm("password")
		info, signedID, expiry, err := manager.Login(c.ClientIP(), username, password)
		if err != nil {
			if !errors.Is(err, auth.ErrInvalidCredentials) && !errors.Is(err, auth.ErrRateLimited) {
				c.Status(http.StatusInternalServerError)
				return
			}
			status := http.StatusUnauthorized
			if errors.Is(err, auth.ErrRateLimited) {
				status = http.StatusTooManyRequests
			}
			c.Status(status)
			renderHTML(c, "login.tmpl", gin.H{"error": "loginFailed", "next": safeNextPath(c.PostForm("next"))})
			return
		}
		_ = info
		http.SetCookie(c.Writer, manager.Cookie(signedID, expiry))
		c.Redirect(http.StatusFound, safeNextPath(c.PostForm("next")))
	})

	protected := router.Group("/")
	protected.Use(authRequired(manager))
	protected.GET("/", func(c *gin.Context) { renderDirectory(c, manager, "") })
	protected.GET("/d/*path", func(c *gin.Context) {
		raw := pathFromParam(c.Param("path"))
		if raw == "" {
			c.Redirect(http.StatusMovedPermanently, appPath("/"))
			return
		}
		renderDirectory(c, manager, raw)
	})
	protected.GET("/view/*path", func(c *gin.Context) {
		absolute, rel, _, err := fileForRead(c.Param("path"))
		if err != nil {
			hiddenNotFound(c)
			return
		}
		setPrivateResponse(c)
		c.Header("Content-Security-Policy", "default-src 'none'; base-uri 'none'; form-action 'none'; sandbox")
		c.Header("X-Content-Type-Options", "nosniff")
		if !isInlinePreviewable(rel) {
			c.Header("Content-Type", "application/octet-stream")
			logAction(c, "查看", rel)
			c.FileAttachment(absolute, filepath.Base(rel))
			return
		}
		setSafeFilePreviewHeaders(c)
		logAction(c, "查看", rel)
		c.File(absolute)
	})
	protected.GET("/batch-download/:ticket", func(c *gin.Context) {
		session := authInfo(c)
		if session.SessionID == "" {
			hiddenNotFound(c)
			return
		}
		items, ok := manager.ConsumeArchiveTicket(session.SessionID, c.Param("ticket"))
		if !ok {
			hiddenNotFound(c)
			return
		}
		selection, err := utils.SelectionFromArchiveItems(items)
		if err != nil {
			errorCode := utils.ErrorCode(err)
			if errorCode == "not_found" || errorCode == "unsupported_file_type" || errorCode == "invalid_path" {
				hiddenNotFound(c)
				return
			}
			c.Status(operationStatus(err))
			return
		}
		archive, cleanup, err := utils.PrepareSelectionZip(selection)
		if err != nil {
			c.Status(operationStatus(err))
			return
		}
		defer cleanup()
		setPrivateResponse(c)
		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", "attachment; filename=goFile-selection.zip")
		c.Header("X-Content-Type-Options", "nosniff")
		_, _ = io.Copy(c.Writer, archive)
	})

	protected.GET("/download/*path", func(c *gin.Context) {
		absolute, rel, _, err := fileForRead(c.Param("path"))
		if err != nil {
			hiddenNotFound(c)
			return
		}
		setPrivateResponse(c)
		c.Header("X-Content-Type-Options", "nosniff")
		logAction(c, "下载", rel)
		c.FileAttachment(absolute, filepath.Base(rel))
	})
	protected.GET("/edit/*path", func(c *gin.Context) {
		absolute, rel, _, err := fileForRead(c.Param("path"))
		if err != nil {
			hiddenNotFound(c)
			return
		}
		data, err := os.ReadFile(absolute)
		if err != nil {
			hiddenNotFound(c)
			return
		}
		logAction(c, "编辑", rel)
		renderHTML(c, "editor.tmpl", gin.H{"data": string(data), "path": rel})
	})
	protected.GET("/api/properties", func(c *gin.Context) {
		if authInfo(c).Bearer {
			setPrivateResponse(c)
			c.JSON(http.StatusForbidden, gin.H{"ok": false, "code": "browser_session_required"})
			return
		}
		setPrivateResponse(c)
		properties, err := utils.GetProperties(c.Query("path"))
		if err != nil {
			jsonError(c, operationStatus(err), err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "properties": properties})
	})

	// Upload APIs are available in normal mode and in -ru mode.
	if !reader || uploader {
		uploadGroup := protected.Group("/")
		uploadGroup.Use(csrfRequired(manager))
		uploadGroup.POST("/do/upload/*path", func(c *gin.Context) {
			upload, err := c.FormFile("file")
			if err != nil {
				jsonError(c, http.StatusBadRequest, utils.ErrInvalidPath)
				return
			}
			destination, rel, err := uploadDestination(pathFromParam(c.Param("path")), upload.Filename)
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			input, err := upload.Open()
			if err != nil {
				jsonError(c, http.StatusInternalServerError, errors.New("io"))
				return
			}
			err = writeUploadedFile(destination, input)
			closeErr := input.Close()
			if err == nil {
				err = closeErr
			}
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			logAction(c, "上传", rel)
			c.JSON(http.StatusOK, gin.H{"ok": true, "path": rel})
		})
		uploadGroup.POST("/do/chunk/check", func(c *gin.Context) {
			dir := chunkDir(c.PostForm("fileId"))
			total, err := strconv.Atoi(c.PostForm("totalChunks"))
			if dir == "" || err != nil || total <= 0 || total > 100000 {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "code": "invalid_upload"})
				return
			}
			uploaded := make([]int, 0)
			for i := 0; i < total; i++ {
				if utils.Exist(filepath.Join(dir, strconv.Itoa(i))) {
					uploaded = append(uploaded, i)
				}
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "uploaded": uploaded})
		})
		uploadGroup.POST("/do/chunk/upload", func(c *gin.Context) {
			dir := chunkDir(c.PostForm("fileId"))
			index, err := strconv.Atoi(c.PostForm("chunkIndex"))
			total, totalErr := strconv.Atoi(c.PostForm("totalChunks"))
			if dir == "" || err != nil || totalErr != nil || index < 0 || total <= 0 || total > 100000 || index >= total {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "code": "invalid_upload"})
				return
			}
			if err := os.MkdirAll(dir, 0700); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "code": "io_error"})
				return
			}
			upload, err := c.FormFile("file")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "code": "invalid_upload"})
				return
			}
			input, err := upload.Open()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "code": "io_error"})
				return
			}
			err = writeUploadedFile(filepath.Join(dir, strconv.Itoa(index)), input)
			closeErr := input.Close()
			if err == nil {
				err = closeErr
			}
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "code": "io_error"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		uploadGroup.POST("/do/chunk/merge", func(c *gin.Context) {
			dir := chunkDir(c.PostForm("fileId"))
			total, err := strconv.Atoi(c.PostForm("totalChunks"))
			if dir == "" || err != nil || total <= 0 || total > 100000 {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "code": "invalid_upload"})
				return
			}
			destination, rel, err := uploadDestination(c.PostForm("path"), c.PostForm("fileName"))
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				jsonError(c, operationStatus(err), utils.ErrDestinationExists)
				return
			}
			success := true
			for i := 0; i < total; i++ {
				input, openErr := os.Open(filepath.Join(dir, strconv.Itoa(i)))
				if openErr != nil {
					success = false
					break
				}
				_, copyErr := io.Copy(output, input)
				inputCloseErr := input.Close()
				if copyErr != nil || inputCloseErr != nil {
					success = false
					break
				}
			}
			closeErr := output.Close()
			_ = os.RemoveAll(dir)
			if !success || closeErr != nil {
				_ = os.Remove(destination)
				jsonError(c, http.StatusInternalServerError, errors.New("io"))
				return
			}
			logAction(c, "上传", rel)
			c.JSON(http.StatusOK, gin.H{"ok": true, "path": rel})
		})
		uploadGroup.POST("/api/upload", func(c *gin.Context) {
			upload, err := c.FormFile("file")
			if err != nil {
				jsonError(c, http.StatusBadRequest, utils.ErrInvalidPath)
				return
			}
			destination, rel, err := uploadDestination(c.PostForm("path"), upload.Filename)
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			input, err := upload.Open()
			if err != nil {
				jsonError(c, http.StatusInternalServerError, errors.New("io"))
				return
			}
			err = writeUploadedFile(destination, input)
			closeErr := input.Close()
			if err == nil {
				err = closeErr
			}
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			logAction(c, "上传", rel)
			c.JSON(http.StatusOK, gin.H{"ok": true, "path": rel})
		})
	}

	// Mutating file management routes do not exist in -r or -ru mode.
	if !reader {
		mutations := protected.Group("/")
		mutations.Use(csrfRequired(manager))
		mutations.POST("/do/newdir", func(c *gin.Context) {
			rel, err := utils.MakeDirectory(c.PostForm("path"), c.PostForm("dirname"))
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			logAction(c, "新建", rel+"/")
			c.JSON(http.StatusOK, gin.H{"ok": true, "path": rel})
		})
		mutations.POST("/do/newfile", func(c *gin.Context) {
			rel, err := utils.MakeFile(c.PostForm("path"), c.PostForm("filename"))
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			logAction(c, "新建", rel)
			c.JSON(http.StatusOK, gin.H{"ok": true, "path": rel})
		})
		mutations.POST("/do/rename", func(c *gin.Context) {
			rel, err := utils.RenameItem(c.PostForm("path"), c.PostForm("name"))
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			logAction(c, "重命名", rel)
			c.JSON(http.StatusOK, gin.H{"ok": true, "path": rel})
		})
		mutations.POST("/do/move", func(c *gin.Context) {
			rel, err := utils.MoveItem(c.PostForm("path"), c.PostForm("destination"), c.PostForm("name"))
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			logAction(c, "移动", rel)
			c.JSON(http.StatusOK, gin.H{"ok": true, "path": rel})
		})
		mutations.POST("/do/copy", func(c *gin.Context) {
			rel, err := utils.CopyItem(c.PostForm("path"), c.PostForm("destination"), c.PostForm("name"))
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			logAction(c, "复制", rel)
			c.JSON(http.StatusOK, gin.H{"ok": true, "path": rel})
		})
		mutations.POST("/do/rm", func(c *gin.Context) {
			absolute, rel, info, err := utils.ResolveExisting(c.PostForm("path"), false)
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
				jsonError(c, http.StatusBadRequest, utils.ErrUnsupportedType)
				return
			}
			if info.IsDir() {
				if err := os.RemoveAll(absolute); err != nil {
					jsonError(c, http.StatusInternalServerError, errors.New("io"))
					return
				}
			} else if err := os.Remove(absolute); err != nil {
				jsonError(c, http.StatusInternalServerError, errors.New("io"))
				return
			}
			logAction(c, "删除", rel)
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		mutations.POST("/do/zip", func(c *gin.Context) {
			rel, err := utils.CreateDirectoryZip(c.PostForm("path"))
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			logAction(c, "压缩", rel)
			c.JSON(http.StatusOK, gin.H{"ok": true, "path": rel})
		})
		mutations.POST("/do/unzip", func(c *gin.Context) {
			rel, err := utils.ExtractArchive(c.PostForm("path"))
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			logAction(c, "解压", rel)
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		mutations.POST("/do/save", func(c *gin.Context) {
			absolute, rel, _, err := fileForRead(c.PostForm("path"))
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			if err := os.WriteFile(absolute, []byte(c.PostForm("data")), 0644); err != nil {
				jsonError(c, http.StatusInternalServerError, errors.New("io"))
				return
			}
			logAction(c, "保存", rel)
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		mutations.POST("/do/md5", func(c *gin.Context) {
			absolute, rel, _, err := fileForRead(c.PostForm("path"))
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			input, err := os.Open(absolute)
			if err != nil {
				jsonError(c, http.StatusInternalServerError, errors.New("io"))
				return
			}
			hash := md5.New()
			_, copyErr := io.Copy(hash, input)
			inputCloseErr := input.Close()
			if copyErr != nil || inputCloseErr != nil {
				jsonError(c, http.StatusInternalServerError, errors.New("io"))
				return
			}
			output := absolute + ".md5"
			value := fmt.Sprintf("%x", hash.Sum(nil))
			checksum, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if errors.Is(err, os.ErrExist) {
				jsonError(c, http.StatusConflict, utils.ErrDestinationExists)
				return
			}
			if err != nil {
				jsonError(c, http.StatusInternalServerError, errors.New("io"))
				return
			}
			_, writeErr := io.WriteString(checksum, value+"  "+filepath.Base(rel)+"\n")
			closeErr := checksum.Close()
			if writeErr != nil || closeErr != nil {
				_ = os.Remove(output)
				jsonError(c, http.StatusInternalServerError, errors.New("io"))
				return
			}
			logAction(c, "新建", rel+".md5")
			c.JSON(http.StatusOK, gin.H{"ok": true, "hash": value})
		})
		mutations.POST("/do/batch/move", func(c *gin.Context) {
			selection, request, err := decodeBatch(c, manager)
			if err == nil {
				_, _, _, err = utils.ResolveDirectory(request.Destination, true)
			}
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			paths, err := utils.BatchMove(selection, request.Destination)
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			logAction(c, "批量移动", fmt.Sprintf("%d 项", len(paths)))
			c.JSON(http.StatusOK, gin.H{"ok": true, "affected": len(paths), "paths": paths})
		})
		mutations.POST("/do/batch/copy", func(c *gin.Context) {
			selection, request, err := decodeBatch(c, manager)
			if err == nil {
				_, _, _, err = utils.ResolveDirectory(request.Destination, true)
			}
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			paths, err := utils.BatchCopy(selection, request.Destination)
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			logAction(c, "批量复制", fmt.Sprintf("%d 项", len(paths)))
			c.JSON(http.StatusOK, gin.H{"ok": true, "affected": len(paths), "paths": paths})
		})
		mutations.POST("/do/batch/delete", func(c *gin.Context) {
			selection, _, err := decodeBatch(c, manager)
			if err != nil {
				jsonError(c, operationStatus(err), err)
				return
			}
			results := utils.BatchDelete(selection)
			allDeleted := true
			for _, result := range results {
				if result.Code != "deleted" {
					allDeleted = false
				}
			}
			logAction(c, "批量删除", fmt.Sprintf("%d 项", len(results)))
			if !allDeleted {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "code": "delete_partial", "items": results})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "affected": len(results), "items": results})
		})
	}

	// A read-only batch ZIP is permitted in all authenticated modes.
	batchArchive := protected.Group("/")
	batchArchive.Use(csrfRequired(manager))
	batchArchive.POST("/do/batch/download-zip", func(c *gin.Context) {
		selection, _, err := decodeBatch(c, manager)
		if err != nil {
			jsonError(c, operationStatus(err), err)
			return
		}
		if err := utils.PreflightSelectionZip(selection); err != nil {
			jsonError(c, operationStatus(err), err)
			return
		}
		items := make([]auth.ArchiveItem, 0, len(selection.Items))
		for _, item := range selection.Items {
			items = append(items, auth.ArchiveItem{Path: item.Relative, Version: utils.EntryVersion(item.Info)})
		}
		ticket, err := manager.IssueArchiveTicket(authInfo(c).SessionID, items)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "download_url": appPath("/batch-download/" + ticket)})
	})
	// Logout is intentionally present in all modes.
	logout := protected.Group("/")
	logout.Use(csrfRequired(manager))
	logout.POST("/logout", func(c *gin.Context) {
		manager.Logout(authInfo(c))
		http.SetCookie(c.Writer, manager.ExpiredCookie())
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	return router
}

func web(manager *auth.Manager) error {
	address := net.JoinHostPort(conf.Host, conf.GoFilePort)
	fmt.Println(strings.Repeat("-", 44))
	fmt.Println("Directory : " + conf.GoFile)
	fmt.Println("Listen    : http://" + address + appPath("/"))
	if !isLoopbackHost(conf.Host) {
		for _, ip := range localIPs() {
			fmt.Println("Access    : http://" + net.JoinHostPort(ip, conf.GoFilePort) + appPath("/"))
		}
	}
	fmt.Println(strings.Repeat("-", 44))
	return http.ListenAndServe(address, withBasePath(newRouter(manager)))
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func parseConfig() (*auth.Manager, error) {
	startup, err := parseStartupConfig(os.Args[1:], os.Getenv, os.Stderr)
	if err != nil {
		return nil, err
	}

	absolute, err := filepath.Abs(startup.Path)
	if err != nil {
		return nil, err
	}
	conf.GoFile = filepath.Clean(absolute)
	conf.GoFilePort = startup.Port
	conf.Host = startup.Host
	reader = startup.ReadOnly
	uploader = startup.UploadReadOnly
	cookieSecure = startup.CookieSecure
	allowInsecureLAN = startup.AllowInsecureLAN
	basePath = startup.BasePath
	if _, err := utils.Root(); err != nil {
		return nil, fmt.Errorf("invalid -path: %w", err)
	}
	return auth.NewManager(startup.Auth)
}

func localIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch value := addr.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			if ip != nil && ip.To4() != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
				ips = append(ips, ip.String())
			}
		}
	}
	return ips
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "hash-password" {
		if err := runHashPasswordCommand(os.Args[2:], terminalPasswordPrompter{file: os.Stdin}, os.Stdout, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}

	manager, err := parseConfig()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := web(manager); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
