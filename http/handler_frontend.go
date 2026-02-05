package http

import (
	"io/fs"
	"net/http"
	"strings"

	"tickets/web"

	"github.com/labstack/echo/v4"
)

type FrontendHandler struct {
	staticFS   fs.FS
	indexFile  string
	fileServer http.Handler
}

func newFrontendHandler() FrontendHandler {
	staticFS, _ := fs.Sub(web.StaticFiles, "client")
	indexFile := "_shell.html"
	return FrontendHandler{
		staticFS:   staticFS,
		indexFile:  indexFile,
		fileServer: http.FileServer(http.FS(staticFS)),
	}
}

func (h FrontendHandler) GetStaticFiles(c echo.Context) error {
	path := c.Request().URL.Path

	// Serve index file for root path
	if path == "/" {
		return h.serveFile(c, h.indexFile)
	}

	// Try to open the file
	cleanPath := strings.TrimPrefix(path, "/")
	f, err := h.staticFS.Open(cleanPath)
	if err != nil {
		// File not found - serve index for SPA routing
		return h.serveFile(c, h.indexFile)
	}
	f.Close()

	// Check if it's a directory
	stat, err := fs.Stat(h.staticFS, cleanPath)
	if err != nil {
		return h.serveFile(c, h.indexFile)
	}

	if stat.IsDir() {
		// Fall back to main index for directories
		return h.serveFile(c, h.indexFile)
	}

	// Serve the actual file
	h.fileServer.ServeHTTP(c.Response(), c.Request())
	return nil
}

func (h FrontendHandler) serveFile(c echo.Context, path string) error {
	content, err := fs.ReadFile(h.staticFS, path)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	contentType := "text/html; charset=utf-8"
	if strings.HasSuffix(path, ".js") {
		contentType = "application/javascript"
	} else if strings.HasSuffix(path, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(path, ".json") {
		contentType = "application/json"
	}

	return c.Blob(http.StatusOK, contentType, content)
}
