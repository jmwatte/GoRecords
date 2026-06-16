package main

import (
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// localFileHandler serves local audio files and cover art securely.
// It intercepts requests under the /media/ prefix and serves files from
// the local filesystem after URL decoding and traversal-prevention checks.
// Only files residing within one of the authorized rootDirs are served.
func localFileHandler(rootDirs ...string) http.Handler {
	// Resolve and clean all authorized roots once.
	authorizedRoots := make([]string, 0, len(rootDirs))
	for _, d := range rootDirs {
		abs, err := filepath.Abs(d)
		if err != nil {
			slog.Warn("localFileHandler: skipping invalid root dir", "dir", d, "error", err)
			continue
		}
		authorizedRoots = append(authorizedRoots, filepath.Clean(abs))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only accept GET requests
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract the relative path after /media/
		rawPath := strings.TrimPrefix(r.URL.Path, "/media/")
		if rawPath == "" || rawPath == r.URL.Path {
			http.NotFound(w, r)
			return
		}

		// URL-decode the path to handle special characters and spaces in filenames
		decoded, err := url.PathUnescape(rawPath)
		if err != nil {
			slog.Warn("failed to decode media path", "path", rawPath, "error", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Clean and resolve to an absolute path (prevents directory traversal)
		cleanPath := filepath.Clean(decoded)
		if !filepath.IsAbs(cleanPath) {
			absPath, err := filepath.Abs(cleanPath)
			if err != nil {
				slog.Warn("invalid media path", "path", cleanPath, "error", err)
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			cleanPath = absPath
		}

		// Security check: the resolved path must be within an authorized root.
		if !isPathAuthorized(cleanPath, authorizedRoots) {
			slog.Warn("path traversal blocked", "path", cleanPath, "authorizedRoots", authorizedRoots)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// Verify the file exists
		info, err := os.Stat(cleanPath)
		if err != nil || info.IsDir() {
			slog.Debug("media file not found", "path", cleanPath)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		// Open the file for serving with Range support
		f, err := os.Open(cleanPath)
		if err != nil {
			slog.Error("failed to open media file", "path", cleanPath, "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		// ServeContent handles Range requests, Content-Type sniffing,
		// Last-Modified, and Content-Disposition automatically.
		http.ServeContent(w, r, filepath.Base(cleanPath), info.ModTime(), f)
	})
}

// isPathAuthorized returns true if the given absolute path resides within
// any of the authorized root directories.
func isPathAuthorized(absPath string, roots []string) bool {
	if len(roots) == 0 {
		return false
	}
	for _, root := range roots {
		// Check if the path starts with the root directory (with separator).
		// This prevents /safe-dir-extra from matching /safe-dir.
		prefix := root
		if !strings.HasSuffix(prefix, string(os.PathSeparator)) {
			prefix += string(os.PathSeparator)
		}
		if strings.HasPrefix(absPath, prefix) || absPath == root {
			return true
		}
	}
	return false
}

// mediaPrefix is the URL prefix the frontend uses to request local media files.
// The frontend should construct URLs like: /media/<url-encoded-absolute-path>
const mediaPrefix = "/media"
