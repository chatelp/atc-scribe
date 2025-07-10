package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/yegors/co-atc/pkg/logger"
)

// StaticFileHandler serves static files dynamically without caching
type StaticFileHandler struct {
	staticDir string
	logger    *logger.Logger
}

// NewStaticFileHandler creates a new static file handler
func NewStaticFileHandler(staticDir string, logger *logger.Logger) *StaticFileHandler {
	return &StaticFileHandler{
		staticDir: staticDir,
		logger:    logger.Named("static-handler"),
	}
}

// ServeHTTP serves static files dynamically
func (h *StaticFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clean the path to prevent directory traversal attacks
	path := filepath.Clean(r.URL.Path)

	// Remove leading slash
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	// If path is empty, serve index.html
	if path == "" {
		path = "index.html"
	}

	// Construct full file path
	fullPath := filepath.Join(h.staticDir, path)

	// Ensure the file is within the static directory (security check)
	absStaticDir, err := filepath.Abs(h.staticDir)
	if err != nil {
		h.logger.Error("Failed to get absolute path for static directory", logger.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		h.logger.Error("Failed to get absolute path for requested file", logger.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(absFullPath, absStaticDir) {
		h.logger.Warn("Attempted directory traversal attack",
			logger.String("requested_path", path),
			logger.String("full_path", absFullPath),
			logger.String("static_dir", absStaticDir))
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Check if file exists
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If it's a directory request without trailing slash, try index.html
			if !strings.HasSuffix(path, "/") {
				indexPath := filepath.Join(fullPath, "index.html")
				if _, indexErr := os.Stat(indexPath); indexErr == nil {
					fullPath = indexPath
					fileInfo, err = os.Stat(fullPath)
				}
			}

			if err != nil {
				h.logger.Debug("File not found", logger.String("path", fullPath))
				http.NotFound(w, r)
				return
			}
		} else {
			h.logger.Error("Failed to stat file", logger.Error(err), logger.String("path", fullPath))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	// Don't serve directories directly
	if fileInfo.IsDir() {
		// Try to serve index.html from the directory
		indexPath := filepath.Join(fullPath, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			fullPath = indexPath
		} else {
			h.logger.Debug("Directory listing not allowed", logger.String("path", fullPath))
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	// Set headers to prevent caching (for dynamic serving)
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Serve the file
	h.logger.Debug("Serving static file",
		logger.String("requested_path", r.URL.Path),
		logger.String("file_path", fullPath))

	http.ServeFile(w, r, fullPath)
}
