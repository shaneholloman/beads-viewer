// Package export provides viewer asset embedding for static site generation.
package export

import (
	"embed"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ViewerAssetsFS embeds the viewer_assets directory for static site export.
// This allows the bv binary to include all necessary HTML/JS/CSS assets
// without requiring them to exist on the filesystem.
//
//go:embed viewer_assets
var ViewerAssetsFS embed.FS

// CopyEmbeddedAssets copies all embedded viewer assets to the specified output directory.
// If title is provided, it replaces "Beads Viewer" in index.html.
func CopyEmbeddedAssets(outputDir, title string) error {
	// Walk the embedded filesystem and copy all files
	return fs.WalkDir(ViewerAssetsFS, "viewer_assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// embed.FS always uses forward slashes, so use strings.TrimPrefix for cross-platform safety
		// (filepath.Rel could have issues on Windows with mixed separators)
		relPath := strings.TrimPrefix(path, "viewer_assets/")
		if relPath == path {
			// This is the root "viewer_assets" directory itself
			return nil
		}

		// Convert to platform-specific path separator for the destination
		destPath := filepath.Join(outputDir, filepath.FromSlash(relPath))

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// Read the embedded file
		content, err := ViewerAssetsFS.ReadFile(path)
		if err != nil {
			return err
		}

		// Special handling for index.html to replace the title
		if relPath == "index.html" && title != "" {
			contentStr := replaceTitle(string(content), title)
			content = []byte(contentStr)
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		// Write the file
		return os.WriteFile(destPath, content, 0644)
	})
}

// replaceTitle replaces the default title in HTML content with the provided title.
// It replaces both the <title> tag and the h1 header.
// The title is HTML-escaped to prevent XSS and broken HTML.
func replaceTitle(content, title string) string {
	if title == "" {
		return content
	}

	// Escape the title for safe HTML insertion
	safeTitle := html.EscapeString(title)

	// Replace title in <title> tag
	content = strings.Replace(content, "<title>Beads Viewer</title>", "<title>"+safeTitle+"</title>", 1)

	// Replace title in h1 header
	content = strings.Replace(content, `<h1 class="text-xl font-semibold">Beads Viewer</h1>`, `<h1 class="text-xl font-semibold">`+safeTitle+`</h1>`, 1)

	return content
}

// HasEmbeddedAssets returns true if viewer assets are embedded in the binary.
func HasEmbeddedAssets() bool {
	// Check if we can read the index.html from the embedded FS
	_, err := ViewerAssetsFS.ReadFile("viewer_assets/index.html")
	return err == nil
}
