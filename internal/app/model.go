package app

import "time"

type URL struct {
	ID            uint64     `json:"id"`
	PathHash      string     `json:"path_hash"`
	LongURL       string     `json:"long_url"`
	ClickCount    int64      `json:"click_count"`
	LongURLSafe   bool       `json:"long_url_safe"`
	CreatedAt     time.Time  `json:"created_at"`
	CreatedBy     string     `json:"created_by"`
	DeletedAt     *time.Time `json:"deleted_at"`
	DeletedBy     *string    `json:"deleted_by"`
	DeletedReason *string    `json:"deleted_reason"`
}

func (u *URL) Columns() []string {
	return []string{
		"id", "path_hash", "long_url", "click_count", "long_url_safe", "created_at",
		"created_by", "deleted_at", "deleted_by", "deleted_reason",
	}
}

func (u *URL) ScanDest() []any {
	return []any{&u.ID, &u.PathHash, &u.LongURL, &u.ClickCount, &u.LongURLSafe,
		&u.CreatedAt, &u.CreatedBy, &u.DeletedAt, &u.DeletedBy, &u.DeletedReason,
	}
}

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"` // e.g., "Malware", "Streaming"
}
