package scanner

import (
	"log/slog"
	"os"
	"path/filepath"
)

// Common cover art filenames, checked in order of preference.
var coverNames = []string{
	"cover.jpg",
	"cover.png",
	"folder.jpg",
	"folder.png",
	"front.jpg",
	"front.png",
	"album.jpg",
	"album.png",
	"Cover.jpg",
	"Folder.jpg",
	"Front.jpg",
	"Album.jpg",
}

// ResolveCoverArt walks up from dir looking for a cover image file.
// It returns:
//   - coverPath: the absolute path to the found image (empty if none found)
//   - albumFolder: the directory that contained the cover image (empty if none found)
func ResolveCoverArt(dir string) (coverPath, albumFolder string) {
	current, err := filepath.Abs(dir)
	if err != nil {
		slog.Warn("resolve cover art: bad directory", "dir", dir, "error", err)
		return "", ""
	}

	for {
		for _, name := range coverNames {
			candidate := filepath.Join(current, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				slog.Debug("resolved cover art",
					"cover", candidate,
					"albumFolder", current,
				)
				return candidate, current
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding anything.
			break
		}
		current = parent
	}

	slog.Debug("no cover art found walking up from", "dir", dir)
	return "", ""
}
