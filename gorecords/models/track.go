package models

type Track struct {
	ID          uint   `gorm:"primaryKey"`
	Path        string `gorm:"uniqueIndex;not null"`
	Title       string
	Artist      string `gorm:"index"`
	AlbumArtist string `gorm:"index"`
	Album       string `gorm:"index"`
	Genre       string `gorm:"index"`
	Year        int    `gorm:"index"`
	TrackNumber int
	DiscNumber  int
	Duration    float64
	CoverPath   string
	AlbumFolder string `gorm:"index"`
}
