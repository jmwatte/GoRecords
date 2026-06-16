package scanner

import (
	"log/slog"
	"os"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gorecords/models"
)

const batchSize = 40

// SyncTracks performs an upsert of all scanned tracks, then prunes rows for
// files that no longer exist on disk. The batch size is capped at 40 to stay
// well under SQLite's 999-variable limit (~13 fields × 40 = 520 vars).
func SyncTracks(db *gorm.DB, tracks []*models.Track) error {
	slog.Info("syncing tracks to database",
		"total", len(tracks),
		"batchSize", batchSize,
	)

	for i := 0; i < len(tracks); i += batchSize {
		end := i + batchSize
		if end > len(tracks) {
			end = len(tracks)
		}
		batch := tracks[i:end]

		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "path"}},
			UpdateAll: true,
		}).CreateInBatches(batch, batchSize).Error; err != nil {
			slog.Error("failed to upsert track batch",
				"batchStart", i,
				"batchEnd", end,
				"error", err,
			)
			return err
		}
	}

	slog.Info("upsert complete", "tracks", len(tracks))
	return nil
}

// PruneMissingTracks queries all tracked paths from the database, checks each
// one against the filesystem, and deletes rows for files that no longer exist.
func PruneMissingTracks(db *gorm.DB) (int64, error) {
	slog.Info("pruning missing tracks")

	var paths []string
	if err := db.Model(&models.Track{}).Pluck("path", &paths).Error; err != nil {
		return 0, err
	}

	var missingIDs []uint
	for _, p := range paths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			var id uint
			if err := db.Model(&models.Track{}).Select("id").
				Where("path = ?", p).Take(&id).Error; err != nil {
				slog.Debug("could not find track id for missing path", "path", p, "error", err)
				continue
			}
			missingIDs = append(missingIDs, id)
		}
	}

	if len(missingIDs) == 0 {
		slog.Info("no missing tracks to prune")
		return 0, nil
	}

	// Delete in batches to avoid SQLite variable limits.
	var deleted int64
	for i := 0; i < len(missingIDs); i += batchSize {
		end := i + batchSize
		if end > len(missingIDs) {
			end = len(missingIDs)
		}
		batch := missingIDs[i:end]

		result := db.Delete(&models.Track{}, batch)
		if result.Error != nil {
			return deleted, result.Error
		}
		deleted += result.RowsAffected
	}

	slog.Info("pruned missing tracks", "removed", deleted)
	return deleted, nil
}

// FullSync runs a complete scan of rootDir, syncs results to the database,
// and prunes entries for files that no longer exist.
func FullSync(db *gorm.DB, rootDir string, emitter ProgressEmitter) error {
	tracks := Scan(rootDir, 0, emitter)
	if err := SyncTracks(db, tracks); err != nil {
		return err
	}
	if _, err := PruneMissingTracks(db); err != nil {
		return err
	}
	slog.Info("full sync complete", "rootDir", rootDir)
	return nil
}
