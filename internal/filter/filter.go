package filter

import "io"

type Filter interface {
	Exists(key string) bool
	Add(key string)
	Serialize() ([]byte, error)
	WriteTo(io.Writer) (int64, error)
}
