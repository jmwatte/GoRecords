package scanner

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorecords/models"

	"github.com/dhowden/tag"
	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/wav"
)

// ExtractTags reads audio metadata from the given file path, resolves
// cover art via the walk-up resolver, and returns a populated Track.
// If the file cannot be opened or has no readable tags, an error is returned
// and the caller should skip the file.
func ExtractTags(filePath string) (*models.Track, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	metadata, err := tag.ReadFrom(f)
	if err != nil {
		return nil, err
	}

	track := &models.Track{
		Path:        filePath,
		Title:       metadata.Title(),
		Artist:      metadata.Artist(),
		AlbumArtist: metadata.AlbumArtist(),
		Album:       metadata.Album(),
		Genre:       metadata.Genre(),
		Year:        metadata.Year(),
	}

	// Track & Disc number
	if tn, total := metadata.Track(); tn != 0 {
		track.TrackNumber = tn
		_ = total
	}
	if dn, total := metadata.Disc(); dn != 0 {
		track.DiscNumber = dn
		_ = total
	}

	// Walk-up cover art resolution from the track's parent directory
	coverPath, albumFolder := ResolveCoverArt(dirFromPath(filePath))
	track.CoverPath = coverPath
	track.AlbumFolder = albumFolder

	// Duration: first try tag Raw() values, then fall back to pure Go decoder.
	track.Duration = durationFromTags(metadata)
	if track.Duration <= 0 {
		// Re-open with a pure Go decoder for formats we support.
		f.Close()
		track.Duration = durationFromDecoder(filePath)
	}

	slog.Debug("extracted tags",
		"path", filePath,
		"title", track.Title,
		"artist", track.Artist,
		"album", track.Album,
		"disc", track.DiscNumber,
		"track", track.TrackNumber,
		"duration", track.Duration,
		"albumFolder", track.AlbumFolder,
	)
	return track, nil
}

// durationFromTags tries to extract duration from the raw tag map.
// Returns 0 if no valid duration tag is found.
func durationFromTags(metadata tag.Metadata) float64 {
	raw := metadata.Raw()

	// Try ID3v2 TLEN (in milliseconds)
	if v, ok := raw["TLEN"]; ok {
		if ms, err := strconv.ParseFloat(toString(v), 64); err == nil && ms > 0 {
			return ms / 1000.0
		}
	}

	// Try MP4 ©len (in milliseconds)
	if v, ok := raw["©len"]; ok {
		if ms, err := strconv.ParseFloat(toString(v), 64); err == nil && ms > 0 {
			return ms / 1000.0
		}
	}

	// Try FLAC / Vorbis TOTALTIME (in milliseconds or seconds)
	if v, ok := raw["TOTALTIME"]; ok {
		s := toString(v)
		// Typically in milliseconds, but some encoders use seconds.
		if ms, err := strconv.ParseFloat(s, 64); err == nil && ms > 0 {
			if ms > 10000 { // Likely milliseconds
				return ms / 1000.0
			}
			return ms
		}
	}

	// Try XMP dm:duration (Adobe / some taggers)
	if v, ok := raw["dm:duration"]; ok {
		if d, err := strconv.ParseFloat(toString(v), 64); err == nil && d > 0 {
			return d
		}
	}

	return 0
}

// durationFromDecoder opens the file with a pure Go decoder to calculate
// duration from the streamer length and sample rate.
func durationFromDecoder(filePath string) float64 {
	f, err := os.Open(filePath)
	if err != nil {
		return 0
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(filePath))

	var streamer beep.StreamSeekCloser
	var format beep.Format

	switch ext {
	case ".mp3":
		s, f, err := mp3.Decode(f)
		if err != nil {
			return 0
		}
		streamer, format = s, f
	case ".flac":
		s, f, err := flac.Decode(f)
		if err != nil {
			return 0
		}
		streamer, format = s, f
	case ".wav":
		s, f, err := wav.Decode(f)
		if err != nil {
			return 0
		}
		streamer, format = s, f
	default:
		return 0
	}
	defer streamer.Close()

	if format.SampleRate == 0 {
		return 0
	}

	duration := float64(streamer.Len()) / float64(format.SampleRate)
	return duration
}

// toString converts a raw tag value to string.
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case time.Duration:
		return strconv.FormatFloat(val.Seconds(), 'f', 0, 64)
	default:
		return strings.TrimSpace(val.(fmt.Stringer).String())
	}
}

// dirFromPath returns the parent directory of the given file path.
func dirFromPath(filePath string) string {
	idx := len(filePath) - 1
	for idx >= 0 && !os.IsPathSeparator(filePath[idx]) {
		idx--
	}
	if idx < 0 {
		return "."
	}
	return filePath[:idx]
}
