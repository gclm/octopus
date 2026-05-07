package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 4,
		Up:      migrateGroupMode,
	})
}

// migrateGroupMode 将 groups.mode 从旧值 (1=轮询, 3=故障转移, 4=加权, 5=健康优先)
// 迁移为新值 (1=故障转移 Fallback, 2=负载均衡 LoadShare)
func migrateGroupMode(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	if !db.Migrator().HasTable("groups") {
		return nil
	}

	// 旧 mode=1 (轮询) → 1 (Fallback)，无需更新
	// 旧 mode=3 (故障转移) → 1 (Fallback)
	if err := db.Exec("UPDATE groups SET mode = 1 WHERE mode = 3").Error; err != nil {
		return fmt.Errorf("failed to migrate group mode 3→1: %w", err)
	}
	// 旧 mode=4 (加权) → 2 (LoadShare)
	if err := db.Exec("UPDATE groups SET mode = 2 WHERE mode = 4").Error; err != nil {
		return fmt.Errorf("failed to migrate group mode 4→2: %w", err)
	}
	// 旧 mode=5 (健康优先) → 2 (LoadShare)
	if err := db.Exec("UPDATE groups SET mode = 2 WHERE mode = 5").Error; err != nil {
		return fmt.Errorf("failed to migrate group mode 5→2: %w", err)
	}

	return nil
}
