package storage

import "errors"

var (
	// This isn't actually an error. It is just a cheap way to bypass writing the whole shit again to disk, handled in the driver.
	ErrCacheHit = errors.New("cache has been hit. this is good btw 👍")
)
