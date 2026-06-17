package scanner

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorecords/models"

	"github.com/abema/go-mp4"
	"github.com/dhowden/tag"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/vorbis"
	"github.com/faiface/beep/wav"
	"github.com/mewkiz/flac"
)

func parseM4ATags(f *os.File) *wavMetadata {
	meta := &wavMetadata{}
	var pendingKey string

	// Map MP4 4-letter codes to your struct's field names
	tagMap := map[string]string{
		"©nam": "title",
		"©ART": "artist",
		"©alb": "album",
		"©gen": "genre",
		"©day": "year",
		"©wrt": "composer", // Note: add to struct if needed
		"aART": "albumArtist",
	}

	if _, err := f.Seek(0, 0); err != nil {
		return nil
	}

	mp4.ReadBoxStructure(f, func(h *mp4.ReadHandle) (interface{}, error) {
		boxType := h.BoxInfo.Type.String()

		// 1. If we hit a known tag box, set our pending key
		if key, ok := tagMap[boxType]; ok {
			pendingKey = key
			return nil, nil
		}

		// 2. If we hit the "data" box, extract the text
		if boxType == "data" && pendingKey != "" {
			payload, _, err := h.ReadPayload()
			if err == nil {
				if d, ok := payload.(*mp4.Data); ok {
					val := strings.TrimRight(string(d.Data), "\x00")

					// Map the string to the correct field in your struct
					switch pendingKey {
					case "title":
						meta.title = val
					case "artist":
						meta.artist = val
					case "album":
						meta.album = val
					case "genre":
						meta.genre = val
					case "albumArtist":
						meta.albumArtist = val
					case "year":
						if y, err := strconv.Atoi(val); err == nil {
							meta.year = y
						}
					}
				}
			}
			pendingKey = ""
			return nil, nil
		}

		// 3. Reset pending key for other boxes
		if pendingKey != "" && boxType != "data" {
			pendingKey = ""
		}

		return nil, nil
	})

	// If we found nothing, return nil
	if meta.title == "" && meta.artist == "" && meta.album == "" {
		return nil
	}

	return meta
}

// ExtractTags reads audio metadata from the given file path, resolves
// cover art via the walk-up resolver, and returns a populated Track.
// If the file cannot be opened or has no readable tags, an error is returned
// and the caller should skip the file.
// The file handle is kept open for both tag reading and duration decoding
// to avoid re-opening races under concurrent scanning.
func ExtractTags(filePath string) (*models.Track, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var metadata tag.Metadata
	var fallback *wavMetadata

	// 1. Try the standard library first
	metadata, err = tag.ReadFrom(f)

	// 2. If it fails, check if it's an unsupported format we can parse manually
	if err != nil {
		ext := strings.ToLower(filepath.Ext(filePath))

		// IMPORTANT: tag.ReadFrom reads bytes, moving the file pointer.
		// We MUST reset it to the beginning for our custom parsers.
		if _, seekErr := f.Seek(0, 0); seekErr != nil {
			return nil, seekErr
		}

		switch ext {
		case ".wav":
			fallback = parseWavInfo(f) // Must return *wavMetadata
		case ".ape":
			fallback = parseApeTags(f) // Must return *wavMetadata
		case ".m4a", ".mp4":
			fallback = parseM4ATags(f) // Must return *wavMetadata
		default:
			slog.Debug("unsupported format, skipping", "path", filePath, "ext", ext, "error", err)
			return nil, err
		}
	}

	track := &models.Track{
		Path: filePath,
	}

	// 3. Map metadata to the track model based on which parser succeeded
	if metadata != nil {
		// Standard dhowden/tag path
		track.Title = metadata.Title()
		track.Artist = metadata.Artist()
		track.AlbumArtist = metadata.AlbumArtist()
		track.Album = metadata.Album()
		track.Genre = metadata.Genre()
		track.Year = metadata.Year()

		if tn, total := metadata.Track(); tn != 0 {
			if tn > 100 && total > 0 && total < 100 {
				track.TrackNumber = tn % 100
			} else {
				track.TrackNumber = tn
			}
		}
		if dn, _ := metadata.Disc(); dn != 0 {
			track.DiscNumber = dn
		}
	} else if fallback != nil {
		// Custom fallback path (M4A, WAV, APE)
		track.Title = fallback.title
		track.Artist = fallback.artist
		track.AlbumArtist = fallback.albumArtist
		track.Album = fallback.album
		track.Genre = fallback.genre
		track.Year = fallback.year
		track.TrackNumber = fallback.trackNumber
		track.DiscNumber = fallback.discNumber
	}

	// Fallback for files where tag library detects format but returns empty metadata
	if track.Title == "" {
		ext := filepath.Ext(filePath)
		track.Title = strings.TrimSuffix(filepath.Base(filePath), ext)
	}
	if track.Album == "" {
		track.Album = filepath.Base(filepath.Dir(filePath))
	}

	// Walk-up cover art resolution
	coverPath, albumFolder := ResolveCoverArt(dirFromPath(filePath))
	track.CoverPath = coverPath
	if albumFolder == "" {
		albumFolder = dirFromPath(filePath)
	}
	track.AlbumFolder = albumFolder

	// Duration logic
	// Note: durationFromTags expects tag.Metadata. If we only have fallback, it will return 0,
	// which is fine because the next line will use the decoder.
	if metadata != nil {
		track.Duration = durationFromTags(metadata)
	}

	if track.Duration <= 0 {
		// CRITICAL: Reset file pointer before decoding
		if _, err := f.Seek(0, 0); err != nil {
			slog.Warn("Failed to seek for duration calculation", "path", filePath, "error", err)
		} else {
			track.Duration = durationFromDecoderWithFile(f, filePath)
		}
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
		if ms, err := strconv.ParseFloat(s, 64); err == nil && ms > 0 {
			if ms > 10000 {
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

// durationFromDecoderWithFile decodes an already-open audio file to calculate
// duration. The caller must have Seek'd to position 0 before calling.
// Errors are logged at Warn level so they're visible in the console.
func durationFromDecoderWithFile(f *os.File, filePath string) float64 {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".flac":
		stream, err := flac.Parse(f)
		if err != nil {
			slog.Warn("duration decoder: flac parse failed", "path", filePath, "error", err)
			return 0
		}
		if stream.Info.NSamples == 0 {
			slog.Warn("duration decoder: flac has 0 samples", "path", filePath)
			return 0
		}
		return float64(stream.Info.NSamples) / float64(stream.Info.SampleRate)

	case ".mp3":
		streamer, format, err := mp3.Decode(f)
		if err != nil {
			slog.Warn("duration decoder: mp3 decode failed", "path", filePath, "error", err)
			return 0
		}
		defer streamer.Close()
		if format.SampleRate == 0 {
			slog.Warn("duration decoder: zero sample rate", "path", filePath)
			return 0
		}
		return float64(streamer.Len()) / float64(format.SampleRate)

	case ".wav":
		streamer, format, err := wav.Decode(f)
		if err != nil {
			slog.Warn("duration decoder: wav decode failed", "path", filePath, "error", err)
			return 0
		}
		defer streamer.Close()
		if format.SampleRate == 0 {
			slog.Warn("duration decoder: zero sample rate", "path", filePath)
			return 0
		}
		return float64(streamer.Len()) / float64(format.SampleRate)

	case ".ogg":
		streamer, format, err := vorbis.Decode(f)
		if err != nil {
			slog.Warn("duration decoder: ogg decode failed", "path", filePath, "error", err)
			return 0
		}
		defer streamer.Close()
		if format.SampleRate == 0 {
			slog.Warn("duration decoder: zero sample rate", "path", filePath)
			return 0
		}
		return float64(streamer.Len()) / float64(format.SampleRate)

	case ".m4a", ".mp4", ".aac":
		return durationFromMP4Container(f, filePath)

	case ".ape":
		return durationFromAPE(f, filePath)

	default:
		slog.Warn("duration decoder: unsupported format", "path", filePath, "ext", ext)
		return 0
	}
}

// durationFromMP4Container parses the MP4/M4A container and extracts duration
// from the Movie Header (mvhd) box.
func durationFromMP4Container(f *os.File, filePath string) float64 {
	if _, err := f.Seek(0, 0); err != nil {
		slog.Warn("duration decoder: failed to seek m4a file", "path", filePath, "error", err)
		return 0
	}

	results, err := mp4.ExtractBoxWithPayload(f, nil, mp4.BoxPath{mp4.StrToBoxType("moov"), mp4.StrToBoxType("mvhd")})
	if err != nil {
		slog.Warn("duration decoder: mp4 parse failed", "path", filePath, "error", err)
		return 0
	}
	if len(results) == 0 {
		slog.Warn("duration decoder: mvhd box not found", "path", filePath)
		return 0
	}

	mvhd, ok := results[0].Payload.(*mp4.Mvhd)
	if !ok {
		slog.Warn("duration decoder: mvhd payload type assertion failed", "path", filePath)
		return 0
	}

	duration := mvhd.GetDuration()
	if mvhd.Timescale == 0 {
		slog.Warn("duration decoder: mp4 has 0 timescale", "path", filePath)
		return 0
	}

	return float64(duration) / float64(mvhd.Timescale)
}

// durationFromAPE parses the APE file header to extract duration.
// For v3.98+ it uses seek-table-based calculation (total frames x blocks/frame).
// For older versions it reads the descriptor at end of file.
func durationFromAPE(f *os.File, filePath string) float64 {
	// Reset file position to beginning
	if _, err := f.Seek(0, 0); err != nil {
		slog.Debug("Failed to seek APE file", "path", filePath, "error", err)
		return 0
	}

	// Read enough for header + descriptor
	buf := make([]byte, 100)
	if _, err := io.ReadFull(f, buf); err != nil {
		slog.Debug("Failed to read APE header", "path", filePath, "error", err)
		return 0
	}

	if string(buf[0:3]) != "MAC" {
		slog.Debug("Invalid APE signature", "path", filePath)
		return 0
	}

	version := binary.LittleEndian.Uint16(buf[4:6])

	if version >= 3980 {
		// v3.98+ — use seek table to calculate duration
		// Fields: [0:4]"MAC " [4:6]version [6:8]compLevel
		//         [8:12]descLen [12:16]headerLen
		//         [16:20]seekTableBytes [20:24]headerDataBytes
		seekTableBytes := binary.LittleEndian.Uint32(buf[16:20])
		compressionLevel := binary.LittleEndian.Uint16(buf[6:8])

		// SampleRate is in the WAVEFORMATEX at the end of the descriptor
		descLen := binary.LittleEndian.Uint32(buf[8:12])
		headerLen := int64(binary.LittleEndian.Uint32(buf[12:16]))
		descTotal := headerLen + int64(descLen)

		// Read the full descriptor if needed
		if descTotal > 100 && descTotal <= 5200 {
			extra := make([]byte, descTotal-100)
			if _, err := io.ReadFull(f, extra); err != nil {
				slog.Debug("Failed to read APE descriptor", "path", filePath, "error", err)
				return 0
			}
			buf = append(buf, extra...)
		}

		// Locate SampleRate (last 4 bytes of descriptor that match a common value)
		var sampleRate uint32
		for offset := descTotal - 4; offset >= headerLen+8; offset -= 4 {
			sr := binary.LittleEndian.Uint32(buf[offset : offset+4])
			if sr == 44100 || sr == 48000 || sr == 88200 || sr == 96000 || sr == 192000 {
				sampleRate = sr
				break
			}
		}
		if sampleRate == 0 {
			slog.Debug("APE: could not find sample rate", "path", filePath)
			return 0
		}

		// Number of frames from seek table (4-byte entries)
		if seekTableBytes == 0 || seekTableBytes%4 != 0 {
			slog.Debug("APE: invalid seek table size", "path", filePath, "bytes", seekTableBytes)
			return 0
		}
		numFrames := int(seekTableBytes / 4)

		// Blocks per frame depends on compression level
		//   fast / default (<=1000) → 73728
		//   normal, high, extra high, insane → 4608
		blocksPerFrame := uint64(4608)
		if compressionLevel <= 1000 {
			blocksPerFrame = 73728
		}

		// Total blocks = (numFrames - 1) * blocksPerFrame + finalFrameBlocks
		// finalFrameBlocks is not easily accessible, so approximate
		totalBlocks := uint64(numFrames-1) * blocksPerFrame
		if totalBlocks == 0 {
			slog.Debug("APE: 0 total blocks", "path", filePath)
			return 0
		}

		return float64(totalBlocks) / float64(sampleRate)
	}

	// Older versions (< v3.98): descriptor at end of file
	fileInfo, err := f.Stat()
	if err != nil {
		slog.Debug("Failed to stat APE file", "path", filePath, "error", err)
		return 0
	}
	descriptorOffset := fileInfo.Size() - 32
	if descriptorOffset < 32 {
		slog.Debug("APE file too small", "path", filePath)
		return 0
	}
	if _, err := f.Seek(descriptorOffset, 0); err != nil {
		slog.Debug("Failed to seek APE descriptor", "path", filePath, "error", err)
		return 0
	}
	desc := make([]byte, 32)
	if _, err := io.ReadFull(f, desc); err != nil {
		slog.Debug("Failed to read APE descriptor", "path", filePath, "error", err)
		return 0
	}
	totalFrames := binary.LittleEndian.Uint32(desc[20:24])
	sampleRate := binary.LittleEndian.Uint32(buf[28:32])
	if totalFrames == 0 || sampleRate == 0 {
		slog.Debug("APE: invalid frames or sample rate", "path", filePath)
		return 0
	}
	return float64(totalFrames) / float64(sampleRate)
}

// ---------------------------------------------------------------------------
// Fallback tag parsers for formats that dhowden/tag does not support
// ---------------------------------------------------------------------------

// wavMetadata is a minimal implementation of tag.Metadata for fallback parsers
// that read WAV RIFF INFO chunks or APEv2 tags manually.
type wavMetadata struct {
	title       string
	artist      string
	albumArtist string
	album       string
	genre       string
	year        int
	trackNumber int
	trackTotal  int
	discNumber  int
	discTotal   int
}

func (m *wavMetadata) Format() tag.Format          { return tag.Format("WAV") }
func (m *wavMetadata) FileType() tag.FileType      { return tag.FileType("WAV") }
func (m *wavMetadata) Title() string               { return m.title }
func (m *wavMetadata) Artist() string              { return m.artist }
func (m *wavMetadata) AlbumArtist() string         { return m.albumArtist }
func (m *wavMetadata) Album() string               { return m.album }
func (m *wavMetadata) Genre() string               { return m.genre }
func (m *wavMetadata) Year() int                   { return m.year }
func (m *wavMetadata) Track() (int, int)           { return m.trackNumber, m.trackTotal }
func (m *wavMetadata) Disc() (int, int)            { return m.discNumber, m.discTotal }
func (m *wavMetadata) Composer() string            { return "" }
func (m *wavMetadata) Picture() *tag.Picture       { return nil }
func (m *wavMetadata) Lyrics() string              { return "" }
func (m *wavMetadata) Comment() string             { return "" }
func (m *wavMetadata) Raw() map[string]interface{} { return nil }

// parseWavInfo extracts basic metadata from a WAV file's RIFF LIST INFO chunk.
// WAV files store metadata in LIST chunks of type INFO with 4-letter codes
// such as INAM (Title), IART (Artist), IPRD (Album), etc.
func parseWavInfo(f *os.File) *wavMetadata {
	meta := &wavMetadata{}

	buf := make([]byte, 8)
	for {
		if _, err := io.ReadFull(f, buf); err != nil {
			break
		}

		chunkID := string(buf[0:4])
		chunkSize := int(binary.LittleEndian.Uint32(buf[4:8]))

		if chunkID == "LIST" {
			listType := make([]byte, 4)
			if _, err := io.ReadFull(f, listType); err != nil {
				break
			}
			if string(listType) == "INFO" {
				remaining := chunkSize - 4
				for remaining > 0 {
					subBuf := make([]byte, 8)
					if _, err := io.ReadFull(f, subBuf); err != nil {
						break
					}
					subID := string(subBuf[0:4])
					subSize := int(binary.LittleEndian.Uint32(subBuf[4:8]))
					remaining -= 8

					strBuf := make([]byte, subSize)
					if _, err := io.ReadFull(f, strBuf); err != nil {
						break
					}
					val := strings.TrimRight(string(strBuf), "\x00")
					remaining -= subSize

					switch subID {
					case "INAM":
						meta.title = val
					case "IART":
						meta.artist = val
					case "IPRD":
						meta.album = val
					case "ICRD":
						meta.year, _ = strconv.Atoi(val)
					case "IGNR":
						meta.genre = val
					case "ITRK":
						meta.trackNumber, _ = strconv.Atoi(val)
					}

					// Chunks are padded to 2-byte boundaries
					if subSize%2 != 0 {
						if _, err := f.Seek(1, 1); err != nil {
							break
						}
						remaining--
					}
				}
			}
			break
		}

		// Skip unknown chunks
		if _, err := f.Seek(int64(chunkSize), 1); err != nil {
			break
		}
		// Skip padding byte if chunk size is odd
		if chunkSize%2 != 0 {
			if _, err := f.Seek(1, 1); err != nil {
				break
			}
		}
	}

	return meta
}

// parseApeTags extracts APEv2 tags from the end of an APE file.
// APEv2 stores a 32-byte footer at the end of the file with the signature
// "APETAGEX", followed by the tag data immediately before it.
func parseApeTags(f *os.File) *wavMetadata {
	meta := &wavMetadata{}

	fileInfo, err := f.Stat()
	if err != nil {
		return meta
	}
	fileSize := fileInfo.Size()

	// APEv2 footer is exactly 32 bytes at the end of the file
	if fileSize < 32 {
		return meta
	}
	if _, err := f.Seek(fileSize-32, 0); err != nil {
		return meta
	}

	footer := make([]byte, 32)
	if _, err := io.ReadFull(f, footer); err != nil {
		return meta
	}

	// Verify APEv2 signature "APETAGEX"
	if string(footer[0:8]) != "APETAGEX" {
		return meta
	}

	version := binary.LittleEndian.Uint32(footer[8:12])
	if version < 2000 {
		return meta // We only support APEv2
	}

	tagSize := binary.LittleEndian.Uint32(footer[12:16])
	itemCount := binary.LittleEndian.Uint32(footer[16:20])
	flags := binary.LittleEndian.Uint32(footer[20:24])

	// The tag data starts right before the 32-byte footer.
	// If bit 31 (0x80000000) is set, a 32-byte header precedes the items.
	hasHeader := (flags & 0x80000000) != 0
	tagStart := fileSize - 32 - int64(tagSize)
	if tagStart < 0 {
		return meta
	}
	if _, err := f.Seek(tagStart, 0); err != nil {
		return meta
	}

	tagData := make([]byte, tagSize)
	if _, err := io.ReadFull(f, tagData); err != nil {
		return meta
	}

	// Parse the items
	offset := 0
	if hasHeader {
		offset = 32 // skip the APETAGEX header
	}
	for i := 0; i < int(itemCount); i++ {
		if offset+8 > len(tagData) {
			break
		}

		itemSize := binary.LittleEndian.Uint32(tagData[offset : offset+4])
		// flags := binary.LittleEndian.Uint32(tagData[offset+4 : offset+8])
		offset += 8

		// Find the null terminator for the key
		keyEnd := offset
		for keyEnd < len(tagData) && tagData[keyEnd] != 0 {
			keyEnd++
		}
		if keyEnd >= len(tagData) {
			break
		}
		key := string(tagData[offset:keyEnd])
		offset = keyEnd + 1 // Skip null terminator

		// Read the value
		if offset+int(itemSize) > len(tagData) {
			break
		}
		value := string(tagData[offset : offset+int(itemSize)])
		offset += int(itemSize)

		// Map APE keys to standard keys
		switch strings.ToUpper(key) {
		case "TITLE":
			meta.title = value
		case "ARTIST":
			meta.artist = value
		case "ALBUM ARTIST":
			meta.albumArtist = value
		case "ALBUMARTIST":
			meta.albumArtist = value
		case "ALBUM":
			meta.album = value
		case "YEAR":
			meta.year, _ = strconv.Atoi(value)
		case "GENRE":
			meta.genre = value
		case "TRACK":
			meta.trackNumber, _ = strconv.Atoi(value)
		}
	}

	return meta
}

// toString converts a raw tag value to string.
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case time.Duration:
		return strconv.FormatFloat(val.Seconds(), 'f', 0, 64)
	case fmt.Stringer:
		return strings.TrimSpace(val.String())
	default:
		return fmt.Sprintf("%v", val)
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
