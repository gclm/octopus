package migrate

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

func init() {
	RegisterBeforeAutoMigration(Migration{
		Version: 3,
		Up:      migrateEndpoints,
	})
}

// codingPlanAnthropicURLs 已知 Coding Plan 渠道的 base_url → Anthropic endpoint URL 映射
var codingPlanAnthropicURLs = map[string]string{
	"https://coding.dashscope.aliyuncs.com/v1":    "https://coding.dashscope.aliyuncs.com/apps/anthropic/v1",
	"https://open.bigmodel.cn/api/coding/paas/v4": "https://open.bigmodel.cn/api/anthropic/v1",
	"https://api.minimaxi.com/v1":                  "https://api.minimaxi.com/anthropic/v1",
}

// 003: 将 channels.type + channels.base_urls 合并为 channels.endpoints (JSON)
func migrateEndpoints(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	dialect := db.Dialector.Name()

	hasColumn := func(table, column string) bool {
		if dialect == "sqlite" {
			var name string
			db.Raw("SELECT name FROM pragma_table_info(?) WHERE name = ? LIMIT 1", table, column).Scan(&name)
			return name == column
		}
		return db.Migrator().HasColumn(table, column)
	}

	hasType := hasColumn("channels", "type")
	hasBaseUrls := hasColumn("channels", "base_urls")
	hasEndpoints := hasColumn("channels", "endpoints")

	// 需要迁移：有旧列且无新列
	if hasType && hasBaseUrls && !hasEndpoints {
		if err := db.Exec("ALTER TABLE channels ADD COLUMN endpoints TEXT").Error; err != nil {
			return fmt.Errorf("failed to add endpoints column: %w", err)
		}

		type row struct {
			ID       int    `gorm:"column:id"`
			Type     int    `gorm:"column:type"`
			BaseUrls string `gorm:"column:base_urls"`
		}
		var rows []row
		if err := db.Raw("SELECT id, type, base_urls FROM channels").Scan(&rows).Error; err != nil {
			return fmt.Errorf("failed to read channels: %w", err)
		}

		type oldBaseUrl struct {
			URL   string `json:"url"`
			Delay int    `json:"delay"`
		}

		for _, r := range rows {
			var urls []oldBaseUrl
			bestUrl := ""
			if r.BaseUrls != "" && r.BaseUrls != "null" {
				json.Unmarshal([]byte(r.BaseUrls), &urls)
				for _, u := range urls {
					if u.URL != "" {
						if bestUrl == "" {
							bestUrl = u.URL
						}
					}
				}
			}

			var endpoints []map[string]any
			endpoints = append(endpoints, map[string]any{
				"type":     r.Type,
				"base_url": bestUrl,
				"enabled":  true,
			})

			// Coding Plan 渠道：自动补充 Anthropic endpoint
			if anthropicURL, ok := codingPlanAnthropicURLs[bestUrl]; ok {
				endpoints = append(endpoints, map[string]any{
					"type":     2, // OutboundTypeAnthropic
					"base_url": anthropicURL,
					"enabled":  true,
				})
			}

			payload, _ := json.Marshal(endpoints)
			if err := db.Exec("UPDATE channels SET endpoints = ? WHERE id = ?", string(payload), r.ID).Error; err != nil {
				return fmt.Errorf("failed to update endpoints for channel %d: %w", r.ID, err)
			}
		}
	}

	// 删除旧列
	dropColumn := func(table, column string) error {
		var sql string
		switch dialect {
		case "sqlite":
			sql = fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column)
		case "mysql":
			sql = fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", table, column)
		case "postgres":
			sql = fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s", table, column)
		default:
			sql = fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column)
		}
		return db.Exec(sql).Error
	}

	if hasType {
		if err := dropColumn("channels", "type"); err != nil {
			return fmt.Errorf("failed to drop channels.type: %w", err)
		}
	}
	if hasBaseUrls {
		if err := dropColumn("channels", "base_urls"); err != nil {
			return fmt.Errorf("failed to drop channels.base_urls: %w", err)
		}
	}

	return nil
}
