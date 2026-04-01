package op

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gclm/octopus/internal/db"
	"github.com/gclm/octopus/internal/model"
)

func prepareRelayLogTest(t *testing.T) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "relay-log-test.db")
	if err := db.InitDB("sqlite", dbPath, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if err := settingRefreshCache(context.Background()); err != nil {
		t.Fatalf("refresh settings: %v", err)
	}
	if err := SettingSetString(model.SettingKeyRelayLogKeepEnabled, "true"); err != nil {
		t.Fatalf("enable relay log keep: %v", err)
	}

	relayLogCacheLock.Lock()
	relayLogCache = make([]model.RelayLog, 0, relayLogMaxSize)
	relayLogCacheLock.Unlock()

	t.Cleanup(func() {
		relayLogCacheLock.Lock()
		relayLogCache = make([]model.RelayLog, 0, relayLogMaxSize)
		relayLogCacheLock.Unlock()
		_ = db.Close()
	})
}

func TestRelayLogAddFlushesWithCanceledRequestContext(t *testing.T) {
	prepareRelayLogTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for i := 0; i < relayLogMaxSize; i++ {
		if err := RelayLogAdd(ctx, model.RelayLog{
			Time:             time.Now().Unix() + int64(i),
			RequestModelName: "gpt-5.4",
			ActualModelName:  "gpt-5.4-xhigh",
			ChannelName:      "test-channel",
			ChannelId:        1,
			UseTime:          123,
		}); err != nil {
			t.Fatalf("relay log add %d: %v", i, err)
		}
	}

	var count int64
	if err := db.GetDB().Model(&model.RelayLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count relay logs: %v", err)
	}
	if count != relayLogMaxSize {
		t.Fatalf("expected %d relay logs persisted, got %d", relayLogMaxSize, count)
	}

	relayLogCacheLock.Lock()
	cached := len(relayLogCache)
	relayLogCacheLock.Unlock()
	if cached != 0 {
		t.Fatalf("expected cache to be flushed, got %d buffered logs", cached)
	}
}
