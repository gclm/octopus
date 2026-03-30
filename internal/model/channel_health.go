package model

type ChannelHealthSummary struct {
	Status              string `json:"status" gorm:"-"`
	BestOrderingScore   int    `json:"best_ordering_score" gorm:"-"`
	WorstRawScore       int    `json:"worst_raw_score" gorm:"-"`
	CooldownRemainingMs int64  `json:"cooldown_remaining_ms" gorm:"-"`
	LastFailureKind     string `json:"last_failure_kind" gorm:"-"`
	TrackedRoutes       int    `json:"tracked_routes" gorm:"-"`
	TrackedKeys         int    `json:"tracked_keys" gorm:"-"`
	CoolingRoutes       int    `json:"cooling_routes" gorm:"-"`
	WarmupRoutes        int    `json:"warmup_routes" gorm:"-"`
}

type ChannelKeyHealthSummary struct {
	Status              string `json:"status" gorm:"-"`
	BestOrderingScore   int    `json:"best_ordering_score" gorm:"-"`
	WorstRawScore       int    `json:"worst_raw_score" gorm:"-"`
	CooldownRemainingMs int64  `json:"cooldown_remaining_ms" gorm:"-"`
	LastFailureKind     string `json:"last_failure_kind" gorm:"-"`
	TrackedRoutes       int    `json:"tracked_routes" gorm:"-"`
	CoolingRoutes       int    `json:"cooling_routes" gorm:"-"`
	WarmupRoutes        int    `json:"warmup_routes" gorm:"-"`
}

type ChannelHealthRoute struct {
	ModelName           string `json:"model_name" gorm:"-"`
	ChannelKeyID        int    `json:"channel_key_id" gorm:"-"`
	State               string `json:"state" gorm:"-"`
	RawScore            int    `json:"raw_score" gorm:"-"`
	OrderingScore       int    `json:"ordering_score" gorm:"-"`
	SuccessCount        int64  `json:"success_count" gorm:"-"`
	WarmupPending       bool   `json:"warmup_pending" gorm:"-"`
	CooldownRemainingMs int64  `json:"cooldown_remaining_ms" gorm:"-"`
	LastFailureKind     string `json:"last_failure_kind" gorm:"-"`
}
