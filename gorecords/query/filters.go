package query

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Op defines the comparison operator for a filter.
type Op string

const (
	OpEq      Op = "="
	OpNeq     Op = "<>"
	OpLt      Op = "<"
	OpGt      Op = ">"
	OpLte     Op = "<="
	OpGte     Op = ">="
	OpLike    Op = "LIKE"
	OpNotLike Op = "NOT LIKE"
	OpIn      Op = "IN"
	OpNotIn   Op = "NOT IN"
	OpIsNull  Op = "IS NULL"
	OpNotNull Op = "IS NOT NULL"
)

// Filter represents a single WHERE constraint.
type Filter struct {
	Field    string      `json:"field"`
	Operator Op          `json:"op"`
	Value    interface{} `json:"value,omitempty"`
}

// Facet holds a distinct value and its count for a given field.
type Facet struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// BuildQuery applies the given filters to a *gorm.DB and returns the chained result.
// Each filter is converted to a parameterised WHERE clause to prevent SQL injection.
func BuildQuery(db *gorm.DB, filters []Filter) *gorm.DB {
	tx := db.Session(&gorm.Session{})

	for _, f := range filters {
		field := safeColumn(f.Field)
		if field == "" {
			continue
		}

		switch f.Operator {
		case OpEq, OpNeq, OpLt, OpGt, OpLte, OpGte:
			tx = tx.Where(fmt.Sprintf("%s %s ?", field, f.Operator), f.Value)

		case OpLike, OpNotLike:
			tx = tx.Where(fmt.Sprintf("%s %s ?", field, f.Operator), f.Value)

		case OpIn, OpNotIn:
			vals, ok := f.Value.([]interface{})
			if !ok {
				// Accept []string etc. by converting through a helper.
				vals = toInterfaceSlice(f.Value)
			}
			if len(vals) > 0 {
				tx = tx.Where(fmt.Sprintf("%s %s ?", field, f.Operator), vals)
			}

		case OpIsNull:
			tx = tx.Where(fmt.Sprintf("%s IS NULL", field))

		case OpNotNull:
			tx = tx.Where(fmt.Sprintf("%s IS NOT NULL", field))
		}
	}

	return tx
}

// CountFacets returns the distinct values and their occurrence counts for the
// given field, constrained by the current set of filters (excluding the field
// being faceted, so the user sees available refinements).
func CountFacets(db *gorm.DB, field string, filters []Filter) ([]Facet, error) {
	field = safeColumn(field)
	if field == "" {
		return nil, fmt.Errorf("query: unsafe column name %q", field)
	}

	// Exclude filters on the same field so the facet reflects available options.
	var filtered []Filter
	for _, f := range filters {
		if f.Field != field {
			filtered = append(filtered, f)
		}
	}

	var facets []Facet
	tx := BuildQuery(db.Model(&struct{}{}), filtered).
		Table("tracks").
		Select(fmt.Sprintf("%s AS value, COUNT(*) AS count", field)).
		Where(fmt.Sprintf("%s IS NOT NULL AND %s != ''", field, field)).
		Group(field).
		Order("count DESC, value ASC")

	if err := tx.Find(&facets).Error; err != nil {
		return nil, err
	}
	return facets, nil
}

// safeColumn validates that a field name is a real column on the tracks table
// to prevent SQL injection via column names.
func safeColumn(name string) string {
	allowed := map[string]bool{
		"id":           true,
		"date_added":   true,
		"path":         true,
		"title":        true,
		"artist":       true,
		"album_artist": true,
		"album":        true,
		"genre":        true,
		"year":         true,
		"track_number": true,
		"disc_number":  true,
		"duration":     true,
		"cover_path":   true,
		"album_folder": true,
	}
	if allowed[name] {
		return name
	}
	return ""
}

// toInterfaceSlice converts any slice type to []interface{}.
func toInterfaceSlice(v interface{}) []interface{} {
	switch s := v.(type) {
	case []interface{}:
		return s
	case []string:
		out := make([]interface{}, len(s))
		for i, x := range s {
			out[i] = x
		}
		return out
	case []int:
		out := make([]interface{}, len(s))
		for i, x := range s {
			out[i] = x
		}
		return out
	case []int64:
		out := make([]interface{}, len(s))
		for i, x := range s {
			out[i] = x
		}
		return out
	case []float64:
		out := make([]interface{}, len(s))
		for i, x := range s {
			out[i] = x
		}
		return out
	default:
		// Try reflection as a last resort.
		return []interface{}{s}
	}
}

// OrderedFilter is a filter tied to a sort direction for album queries.
type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

// AlbumQuery holds pagination, filters, and sort for album-level queries.
type AlbumQuery struct {
	Filters []Filter  `json:"filters"`
	SortBy  string    `json:"sortBy"`
	SortDir SortOrder `json:"sortDir"`
	Offset  int       `json:"offset"`
	Limit   int       `json:"limit"`
}

// GenerateFacets computes facet counts for multiple fields in one operation each.
// Given a list of field names, it returns a map keyed by field name containing
// the distinct values and their occurrence counts under the current filters.
func GenerateFacets(db *gorm.DB, fields []string, filters []Filter) (map[string][]Facet, error) {
	result := make(map[string][]Facet, len(fields))
	for _, field := range fields {
		facets, err := CountFacets(db, field, filters)
		if err != nil {
			return nil, err
		}
		result[field] = facets
	}
	return result, nil
}

// AlbumResult represents a single aggregated album row from the query.
type AlbumResult struct {
	AlbumFolder   string  `json:"albumFolder"`
	DateAdded     string  `json:"dateAdded"`
	Album         string  `json:"album"`
	AlbumArtist   string  `json:"albumArtist"`
	CoverPath     string  `json:"coverPath"`
	Year          int     `json:"year"`
	Genre         string  `json:"genre"`
	TrackCount    int64   `json:"trackCount"`
	TotalDuration float64 `json:"totalDuration"`
}

// PaginatedAlbums wraps a page of results with the total count.
type PaginatedAlbums struct {
	Albums []AlbumResult `json:"albums"`
	Total  int64         `json:"total"`
	Offset int           `json:"offset"`
	Limit  int           `json:"limit"`
}

// GetAlbumsPaginated returns a paginated, aggregated list of albums grouped by
// album_folder. It returns at most `limit` albums at a time, starting at `offset`.
// Set limit to 0 to use the default page size of 100.
func GetAlbumsPaginated(db *gorm.DB, q AlbumQuery) (*PaginatedAlbums, error) {
	if q.Limit <= 0 {
		q.Limit = 100
	}

	// Count total matching albums
	var total int64
	countTx := db.Model(&struct{}{}).
		Table("tracks").
		Where("album_folder IS NOT NULL AND album_folder != ''")
	countTx = BuildQuery(countTx, q.Filters)
	if err := countTx.Select("COUNT(DISTINCT album_folder)").Scan(&total).Error; err != nil {
		return nil, err
	}

	// Fetch the page
	var results []AlbumResult
	tx := BuildAlbumQuery(db, q)
	if err := tx.Find(&results).Error; err != nil {
		return nil, err
	}

	if results == nil {
		results = []AlbumResult{}
	}

	return &PaginatedAlbums{
		Albums: results,
		Total:  total,
		Offset: q.Offset,
		Limit:  q.Limit,
	}, nil
}

// GetRandomAlbum returns a single random album (aggregated by album_folder)
// that matches the given filters. Uses SQLite's RANDOM() for efficient
// server-side randomization.
func GetRandomAlbum(db *gorm.DB, filters []Filter) (*AlbumResult, error) {
	tx := db.Model(&struct{}{}).
		Table("tracks").
		Select(strings.Join([]string{
			"album_folder",
			"MAX(album) AS album",
			"MAX(album_artist) AS album_artist",
			"MAX(cover_path) AS cover_path",
			"MAX(year) AS year",
			"MAX(genre) AS genre",
			"COUNT(*) AS track_count",
			"SUM(duration) AS total_duration",
		}, ", ")).
		Where("album_folder IS NOT NULL AND album_folder != ''")
	tx = BuildQuery(tx, filters)
	tx = tx.Group("album_folder").
		Order("RANDOM()").
		Limit(1)

	var result AlbumResult
	if err := tx.Take(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

// BuildAlbumQuery builds a query that groups tracks by album_folder, applies
// filters, and returns album-level results with pagination.
func BuildAlbumQuery(db *gorm.DB, q AlbumQuery) *gorm.DB {
	tx := db.Model(&struct{}{}).
		Table("tracks").
		Select(strings.Join([]string{
			"album_folder",
			"MAX(date_added) AS date_added",
			"MAX(album) AS album",
			"MAX(album_artist) AS album_artist",
			"MAX(cover_path) AS cover_path",
			"MAX(year) AS year",
			"MAX(genre) AS genre",
			"COUNT(*) AS track_count",
			"SUM(duration) AS total_duration",
		}, ", ")).
		Where("album_folder IS NOT NULL AND album_folder != ''")
	tx = BuildQuery(tx, q.Filters)
	tx = tx.Group("album_folder")

	sortCol := safeColumn(q.SortBy)
	if sortCol != "" {
		dir := string(q.SortDir)
		if dir != "ASC" && dir != "DESC" {
			dir = "ASC"
		}
		tx = tx.Order(fmt.Sprintf("MAX(%s) %s", sortCol, dir))
	} else {
		tx = tx.Order("MAX(album) ASC")
	}

	tx = tx.Offset(q.Offset)
	if q.Limit > 0 {
		tx = tx.Limit(q.Limit)
	}

	return tx
}
