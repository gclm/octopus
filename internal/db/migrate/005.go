package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 5,
		Up:      addRelayLogIndexes,
	})
}

func addRelayLogIndexes(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	if !db.Migrator().HasTable("relay_logs") {
		return nil
	}

	indexes := []struct {
		name string
		sql  string
	}{
		{"idx_relay_logs_time", "CREATE INDEX IF NOT EXISTS idx_relay_logs_time ON relay_logs(time DESC)"},
		{"idx_relay_logs_model", "CREATE INDEX IF NOT EXISTS idx_relay_logs_model ON relay_logs(request_model_name)"},
		{"idx_relay_logs_channel", "CREATE INDEX IF NOT EXISTS idx_relay_logs_channel ON relay_logs(channel_id)"},
		{"idx_relay_logs_request_id", "CREATE INDEX IF NOT EXISTS idx_relay_logs_request_id ON relay_logs(request_id)"},
		{"idx_relay_logs_client_request_id", "CREATE INDEX IF NOT EXISTS idx_relay_logs_client_request_id ON relay_logs(client_request_id)"},
	}

	for _, idx := range indexes {
		if err := db.Exec(idx.sql).Error; err != nil {
			return fmt.Errorf("failed to create index %s: %w", idx.name, err)
		}
	}

	return nil
}
