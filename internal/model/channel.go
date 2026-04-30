package model

import (
	"time"

	"github.com/gclm/octopus/internal/transformer/inbound"
	"github.com/gclm/octopus/internal/transformer/outbound"
)

type AutoGroupType int

const (
	AutoGroupTypeNone  AutoGroupType = 0 //不自动分组
	AutoGroupTypeFuzzy AutoGroupType = 1 //模糊匹配
	AutoGroupTypeExact AutoGroupType = 2 //准确匹配
	AutoGroupTypeRegex AutoGroupType = 3 //正则匹配
)

type Endpoint struct {
	Type    outbound.OutboundType `json:"type"`
	BaseUrl string                `json:"base_url"`
	Enabled bool                  `json:"enabled"`
}

type Channel struct {
	ID            int                    `json:"id" gorm:"primaryKey"`
	Name          string                 `json:"name" gorm:"unique;not null"`
	Endpoints     []Endpoint             `json:"endpoints" gorm:"serializer:json"`
	Keys          []ChannelKey           `json:"keys" gorm:"foreignKey:ChannelID"`
	Model         string                 `json:"model"`
	CustomModel   string                 `json:"custom_model"`
	Enabled       bool                   `json:"enabled" gorm:"default:true"`
	Proxy         bool                   `json:"proxy" gorm:"default:false"`
	AutoSync      bool                   `json:"auto_sync" gorm:"default:false"`
	AutoGroup     AutoGroupType          `json:"auto_group" gorm:"default:0"`
	CustomHeader  []CustomHeader         `json:"custom_header" gorm:"serializer:json"`
	ParamOverride *string                `json:"param_override"`
	ChannelProxy  *string                `json:"channel_proxy"`
	Stats         *StatsChannel          `json:"stats,omitempty" gorm:"foreignKey:ChannelID"`
	MatchRegex    *string                `json:"match_regex"`
}

type CustomHeader struct {
	HeaderKey   string `json:"header_key"`
	HeaderValue string `json:"header_value"`
}

type ChannelKey struct {
	ID               int     `json:"id" gorm:"primaryKey"`
	ChannelID        int     `json:"channel_id"`
	Enabled          bool    `json:"enabled" gorm:"default:true"`
	ChannelKey       string  `json:"channel_key"`
	StatusCode       int     `json:"status_code"`
	LastUseTimeStamp int64   `json:"last_use_time_stamp"`
	TotalCost        float64 `json:"total_cost"`
	Remark           string  `json:"remark"`
}

// ChannelUpdateRequest 渠道更新请求 - 仅包含变更的数据
type ChannelUpdateRequest struct {
	ID            int                    `json:"id" binding:"required"`
	Name          *string                `json:"name,omitempty"`
	Endpoints     *[]Endpoint            `json:"endpoints,omitempty"`
	Enabled       *bool                  `json:"enabled,omitempty"`
	Model         *string                `json:"model,omitempty"`
	CustomModel   *string                `json:"custom_model,omitempty"`
	Proxy         *bool                  `json:"proxy,omitempty"`
	AutoSync      *bool                  `json:"auto_sync,omitempty"`
	AutoGroup     *AutoGroupType         `json:"auto_group,omitempty"`
	CustomHeader  *[]CustomHeader        `json:"custom_header,omitempty"`
	ChannelProxy  *string                `json:"channel_proxy,omitempty"`
	ParamOverride *string                `json:"param_override,omitempty"`
	MatchRegex    *string                `json:"match_regex,omitempty"`

	KeysToAdd    []ChannelKeyAddRequest    `json:"keys_to_add,omitempty"`
	KeysToUpdate []ChannelKeyUpdateRequest `json:"keys_to_update,omitempty"`
	KeysToDelete []int                     `json:"keys_to_delete,omitempty"`
}

type ChannelKeyAddRequest struct {
	Enabled    bool   `json:"enabled"`
	ChannelKey string `json:"channel_key" binding:"required"`
	Remark     string `json:"remark"`
}

type ChannelKeyUpdateRequest struct {
	ID         int     `json:"id" binding:"required"`
	Enabled    *bool   `json:"enabled,omitempty"`
	ChannelKey *string `json:"channel_key,omitempty"`
	Remark     *string `json:"remark,omitempty"`
}

// ChannelFetchModelRequest is used by /channel/fetch-model (not persisted).
type ChannelFetchModelRequest struct {
	Endpoints     []Endpoint     `json:"endpoints" binding:"required"`
	Key           string         `json:"key" binding:"required"`
	Proxy         bool           `json:"proxy"`
	CustomHeader  []CustomHeader `json:"custom_header,omitempty"`
}

func inboundToOutbound(inType inbound.InboundType) outbound.OutboundType {
	switch inType {
	case inbound.InboundTypeOpenAIChat:
		return outbound.OutboundTypeOpenAIChat
	case inbound.InboundTypeOpenAIResponse:
		return outbound.OutboundTypeOpenAIResponse
	case inbound.InboundTypeAnthropic:
		return outbound.OutboundTypeAnthropic
	case inbound.InboundTypeGemini:
		return outbound.OutboundTypeGemini
	case inbound.InboundTypeOpenAIEmbedding:
		return outbound.OutboundTypeOpenAIEmbedding
	default:
		return outbound.OutboundTypeOpenAIChat
	}
}

// MatchEndpoint 根据 inbound 协议类型匹配最佳 endpoint
// 返回 (endpoint, exactMatch)
// exactMatch=true 表示协议完全匹配，无需转换
func (c *Channel) MatchEndpoint(inboundType inbound.InboundType) (*Endpoint, bool) {
	if c == nil {
		return nil, false
	}
	outboundType := inboundToOutbound(inboundType)

	// 1. 精确匹配：入站协议 == 出站协议，零转换
	for i := range c.Endpoints {
		if c.Endpoints[i].Enabled && c.Endpoints[i].Type == outboundType {
			return &c.Endpoints[i], true
		}
	}

	// 2. 兼容匹配：取第一个 enabled 的 endpoint（需协议转换）
	for i := range c.Endpoints {
		if c.Endpoints[i].Enabled {
			return &c.Endpoints[i], false
		}
	}

	return nil, false
}

// GetBaseUrl 返回第一个 enabled endpoint 的 base_url
func (c *Channel) GetBaseUrl() string {
	if c == nil {
		return ""
	}
	for _, ep := range c.Endpoints {
		if ep.Enabled && ep.BaseUrl != "" {
			return ep.BaseUrl
		}
	}
	return ""
}

// KeyFilter 用于过滤 key 的函数类型
type KeyFilter func(key ChannelKey) bool

// GetChannelKey 获取一个可用的 channel key
// filters 是可选的过滤器列表，返回 false 表示跳过该 key
func (c *Channel) GetChannelKey(filters ...KeyFilter) ChannelKey {
	if c == nil || len(c.Keys) == 0 {
		return ChannelKey{}
	}

	nowSec := time.Now().Unix()

	best := ChannelKey{}
	bestCost := 0.0
	bestSet := false

	for _, k := range c.Keys {
		if !k.Enabled || k.ChannelKey == "" {
			continue
		}
		// 429 冷却检查
		if k.StatusCode == 429 && k.LastUseTimeStamp > 0 {
			if nowSec-k.LastUseTimeStamp < int64(5*time.Minute/time.Second) {
				continue
			}
		}
		// 应用自定义过滤器
		skip := false
		for _, filter := range filters {
			if !filter(k) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		if !bestSet || k.TotalCost < bestCost {
			best = k
			bestCost = k.TotalCost
			bestSet = true
		}
	}

	if !bestSet {
		return ChannelKey{}
	}
	return best
}
