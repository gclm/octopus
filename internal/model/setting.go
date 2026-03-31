package model

import (
	"fmt"
	"net/url"
	"strconv"
)

type SettingKey string

const (
	SettingKeyProxyURL                                      SettingKey = "proxy_url"
	SettingKeyStatsSaveInterval                             SettingKey = "stats_save_interval"                                 // 将统计信息写入数据库的周期(分钟)
	SettingKeyModelInfoUpdateInterval                       SettingKey = "model_info_update_interval"                          // 模型信息更新间隔(小时)
	SettingKeySyncLLMInterval                               SettingKey = "sync_llm_interval"                                   // LLM 同步间隔(小时)
	SettingKeyRelayLogKeepPeriod                            SettingKey = "relay_log_keep_period"                               // 日志保存时间范围(天)
	SettingKeyRelayLogKeepEnabled                           SettingKey = "relay_log_keep_enabled"                              // 是否保留历史日志
	SettingKeyCORSAllowOrigins                              SettingKey = "cors_allow_origins"                                  // 跨域白名单(逗号分隔, 如 "example.com,example2.com"). 为空不允许跨域, "*"允许所有
	SettingKeyCircuitBreakerThreshold                       SettingKey = "circuit_breaker_threshold"                           // 熔断触发阈值（连续失败次数）
	SettingKeyCircuitBreakerCooldown                        SettingKey = "circuit_breaker_cooldown"                            // 熔断基础冷却时间（秒）
	SettingKeyCircuitBreakerMaxCooldown                     SettingKey = "circuit_breaker_max_cooldown"                        // 熔断最大冷却时间（秒），指数退避上限
	SettingKeyCircuitBreakerHealthScoreThreshold            SettingKey = "circuit_breaker_health_score_threshold"              // 健康分阈值，低于该值会触发更长冷却
	SettingKeyCircuitBreakerHealthScoreMin                  SettingKey = "circuit_breaker_health_score_min"                    // 健康分下限
	SettingKeyCircuitBreakerHealthScoreMax                  SettingKey = "circuit_breaker_health_score_max"                    // 健康分上限
	SettingKeyCircuitBreakerHealthScoreDecayStep            SettingKey = "circuit_breaker_health_score_decay_step"             // 每次衰减步长
	SettingKeyCircuitBreakerHealthScoreDecayIntervalSeconds SettingKey = "circuit_breaker_health_score_decay_interval_seconds" // 健康分衰减周期（秒）
	SettingKeyCircuitBreakerHealthScoreWarmupSuccesses      SettingKey = "circuit_breaker_health_score_warmup_successes"       // 正向健康分生效前所需的成功样本数
	SettingKeyCircuitBreakerExplorationEvery                SettingKey = "circuit_breaker_exploration_every"                  // 低频探索间隔（每 N 次请求触发一次探索）
)

type Setting struct {
	Key   SettingKey `json:"key" gorm:"primaryKey"`
	Value string     `json:"value" gorm:"not null"`
}

func DefaultSettings() []Setting {
	return []Setting{
		{Key: SettingKeyProxyURL, Value: ""},
		{Key: SettingKeyStatsSaveInterval, Value: "10"},          // 默认10分钟保存一次统计信息
		{Key: SettingKeyCORSAllowOrigins, Value: ""},             // CORS 默认不允许跨域，设置为 "*" 才允许所有来源
		{Key: SettingKeyModelInfoUpdateInterval, Value: "24"},    // 默认24小时更新一次模型信息
		{Key: SettingKeySyncLLMInterval, Value: "24"},            // 默认24小时同步一次LLM
		{Key: SettingKeyRelayLogKeepPeriod, Value: "7"},          // 默认日志保存7天
		{Key: SettingKeyRelayLogKeepEnabled, Value: "true"},      // 默认保留历史日志
		{Key: SettingKeyCircuitBreakerThreshold, Value: "5"},     // 默认连续失败5次触发熔断
		{Key: SettingKeyCircuitBreakerCooldown, Value: "60"},     // 默认基础冷却60秒
		{Key: SettingKeyCircuitBreakerMaxCooldown, Value: "600"}, // 默认最大冷却600秒（10分钟）
		{Key: SettingKeyCircuitBreakerHealthScoreThreshold, Value: "-50"},
		{Key: SettingKeyCircuitBreakerHealthScoreMin, Value: "-100"},
		{Key: SettingKeyCircuitBreakerHealthScoreMax, Value: "100"},
		{Key: SettingKeyCircuitBreakerHealthScoreDecayStep, Value: "5"},
		{Key: SettingKeyCircuitBreakerHealthScoreDecayIntervalSeconds, Value: "600"},
		{Key: SettingKeyCircuitBreakerHealthScoreWarmupSuccesses, Value: "3"},
		{Key: SettingKeyCircuitBreakerExplorationEvery, Value: "6"},
	}
}

func (s *Setting) Validate() error {
	switch s.Key {
	case SettingKeyModelInfoUpdateInterval, SettingKeySyncLLMInterval, SettingKeyRelayLogKeepPeriod,
		SettingKeyCircuitBreakerThreshold, SettingKeyCircuitBreakerCooldown, SettingKeyCircuitBreakerMaxCooldown,
		SettingKeyCircuitBreakerHealthScoreThreshold, SettingKeyCircuitBreakerHealthScoreMin, SettingKeyCircuitBreakerHealthScoreMax,
		SettingKeyCircuitBreakerHealthScoreDecayStep, SettingKeyCircuitBreakerHealthScoreDecayIntervalSeconds,
		SettingKeyCircuitBreakerHealthScoreWarmupSuccesses, SettingKeyCircuitBreakerExplorationEvery:
		_, err := strconv.Atoi(s.Value)
		if err != nil {
			return fmt.Errorf("model info update interval must be an integer")
		}
		return nil
	case SettingKeyRelayLogKeepEnabled:
		if s.Value != "true" && s.Value != "false" {
			return fmt.Errorf("relay log keep enabled must be true or false")
		}
		return nil
	case SettingKeyProxyURL:
		if s.Value == "" {
			return nil
		}
		parsedURL, err := url.Parse(s.Value)
		if err != nil {
			return fmt.Errorf("proxy URL is invalid: %w", err)
		}
		validSchemes := map[string]bool{
			"http":   true,
			"https":  true,
			"socks5": true,
		}
		if !validSchemes[parsedURL.Scheme] {
			return fmt.Errorf("proxy URL scheme must be http, https, socks, or socks5")
		}
		if parsedURL.Host == "" {
			return fmt.Errorf("proxy URL must have a host")
		}
		return nil
	}

	return nil
}
