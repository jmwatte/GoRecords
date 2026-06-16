package models

type Track struct {
	ID          uint    `gorm:"primaryKey" json:"id"`
	Path        string  `gorm:"uniqueIndex;not null" json:"path"`
	Title       string  `json:"title"`
	Artist      string  `gorm:"index" json:"artist"`
	AlbumArtist string  `gorm:"index" json:"albumArtist"`
	Album       string  `gorm:"index" json:"album"`
	Genre       string  `gorm:"index" json:"genre"`
	Year        int     `gorm:"index" json:"year"`
	TrackNumber int     `json:"trackNumber"`
	DiscNumber  int     `json:"discNumber"`
	Duration    float64 `json:"duration"`
	CoverPath   string  `json:"coverPath"`
	AlbumFolder string  `gorm:"index" json:"albumFolder"`
}
