package model

type GroupMode int

const (
	GroupModeFallback  GroupMode = 1 // 故障转移：按优先级排序，同优先级内轮询
	GroupModeLoadShare GroupMode = 2 // 负载均衡：按权重分配，自动结合健康分降级
)

// legacyToNewMode 旧模式到新模式的映射
var legacyToNewMode = map[GroupMode]GroupMode{
	1: GroupModeFallback,  // 轮询 → 故障转移
	3: GroupModeFallback,  // 故障转移 → 故障转移
	4: GroupModeLoadShare, // 加权分配 → 负载均衡
	5: GroupModeLoadShare, // 健康优先 → 负载均衡
}

// NormalizeGroupMode 将旧模式值映射为新模式值
func NormalizeGroupMode(mode GroupMode) GroupMode {
	if newMode, ok := legacyToNewMode[mode]; ok {
		return newMode
	}
	return GroupModeFallback // 默认
}

type Group struct {
	ID                int         `json:"id" gorm:"primaryKey"`
	Name              string      `json:"name" gorm:"unique;not null"`
	Mode              GroupMode   `json:"mode" gorm:"not null"`
	MatchRegex        string      `json:"match_regex"`
	FirstTokenTimeOut int         `json:"first_token_time_out"` // 单个渠道首个Token响应超时时间(秒)
	SessionKeepTime   int         `json:"session_keep_time"`    // 会话保持时间(秒) 0 为禁用
	Items             []GroupItem `json:"items,omitempty" gorm:"foreignKey:GroupID"`
}

type GroupItem struct {
	ID        int    `json:"id" gorm:"primaryKey"`
	GroupID   int    `json:"group_id" gorm:"not null;index:idx_group_channel_model,unique"` // 创建时不携带此字段,更新时需要
	ChannelID int    `json:"channel_id" gorm:"not null;index:idx_group_channel_model,unique"`
	ModelName string `json:"model_name" gorm:"not null;index:idx_group_channel_model,unique"`
	Priority  int    `json:"priority"`
	Weight    int    `json:"weight"`
}

// GroupUpdateRequest 分组更新请求 - 仅包含变更的数据
type GroupUpdateRequest struct {
	ID                int                      `json:"id" binding:"required"`
	Name              *string                  `json:"name,omitempty"`                 // 仅在名称变更时发送
	Mode              *GroupMode               `json:"mode,omitempty"`                 // 仅在模式变更时发送
	MatchRegex        *string                  `json:"match_regex,omitempty"`          // 仅在匹配正则变更时发送
	FirstTokenTimeOut *int                     `json:"first_token_time_out,omitempty"` // 仅在超时变更时发送(秒)
	SessionKeepTime   *int                     `json:"session_keep_time,omitempty"`    // 仅在会话保持时间变更时发送(秒)
	ItemsToAdd        []GroupItemAddRequest    `json:"items_to_add,omitempty"`         // 新增的 items
	ItemsToUpdate     []GroupItemUpdateRequest `json:"items_to_update,omitempty"`      // 更新的 items (priority 变更)
	ItemsToDelete     []int                    `json:"items_to_delete,omitempty"`      // 删除的 item IDs
}

// GroupItemAddRequest 新增 item 请求
type GroupItemAddRequest struct {
	ChannelID int    `json:"channel_id" binding:"required"`
	ModelName string `json:"model_name" binding:"required"`
	Priority  int    `json:"priority,omitempty"`
	Weight    int    `json:"weight,omitempty"`
}

// GroupItemUpdateRequest 更新 item 请求
type GroupItemUpdateRequest struct {
	ID       int `json:"id" binding:"required"`
	Priority int `json:"priority,omitempty"`
	Weight   int `json:"weight,omitempty"`
}
type GroupIDAndLLMName struct {
	ChannelID int
	ModelName string
}
