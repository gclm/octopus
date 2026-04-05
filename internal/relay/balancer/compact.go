package balancer

import (
	"fmt"
	"sync"
	"time"
)

type compactUnsupportedEntry struct {
	Timestamp time.Time
}

var (
	compactUnsupportedCache sync.Map
	compactUnsupportedTTL   = defaultCompactUnsupportedTTL
)

const defaultCompactUnsupportedTTL = 10 * time.Minute

func compactUnsupportedKey(channelID int, modelName string) string {
	return fmt.Sprintf("%d:%s", channelID, modelName)
}

func IsCompactUnsupported(channelID int, modelName string) (unsupported bool, remaining time.Duration) {
	key := compactUnsupportedKey(channelID, modelName)
	value, ok := compactUnsupportedCache.Load(key)
	if !ok {
		return false, 0
	}
	entry := value.(*compactUnsupportedEntry)
	elapsed := time.Since(entry.Timestamp)
	if elapsed >= compactUnsupportedTTL {
		compactUnsupportedCache.Delete(key)
		return false, 0
	}
	return true, compactUnsupportedTTL - elapsed
}

func MarkCompactUnsupported(channelID int, modelName string) {
	compactUnsupportedCache.Store(compactUnsupportedKey(channelID, modelName), &compactUnsupportedEntry{
		Timestamp: time.Now(),
	})
}

func MarkCompactSupported(channelID int, modelName string) {
	compactUnsupportedCache.Delete(compactUnsupportedKey(channelID, modelName))
}
