package model

import (
	"testing"
	"time"

	"github.com/gclm/octopus/internal/transformer/inbound"
	"github.com/gclm/octopus/internal/transformer/outbound"
)

func TestChannel_GetChannelKey(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name          string
		channel       *Channel
		filters       []KeyFilter
		expectEmpty   bool
		expectedKeyID int
	}{
		{
			name:        "nil channel returns empty",
			channel:     nil,
			expectEmpty: true,
		},
		{
			name:        "channel with no keys returns empty",
			channel:     &Channel{ID: 1, Name: "test"},
			expectEmpty: true,
		},
		{
			name: "channel with disabled key returns empty",
			channel: &Channel{
				ID:   1,
				Name: "test",
				Keys: []ChannelKey{
					{ID: 1, ChannelID: 1, Enabled: false, ChannelKey: "key1"},
				},
			},
			expectEmpty: true,
		},
		{
			name: "channel with empty key string returns empty",
			channel: &Channel{
				ID:   1,
				Name: "test",
				Keys: []ChannelKey{
					{ID: 1, ChannelID: 1, Enabled: true, ChannelKey: ""},
				},
			},
			expectEmpty: true,
		},
		{
			name: "selects key with lowest cost",
			channel: &Channel{
				ID:   1,
				Name: "test",
				Keys: []ChannelKey{
					{ID: 1, ChannelID: 1, Enabled: true, ChannelKey: "key1", TotalCost: 10.0},
					{ID: 2, ChannelID: 1, Enabled: true, ChannelKey: "key2", TotalCost: 5.0},
					{ID: 3, ChannelID: 1, Enabled: true, ChannelKey: "key3", TotalCost: 8.0},
				},
			},
			expectEmpty:   false,
			expectedKeyID: 2, // key2 has lowest cost
		},
		{
			name: "skips key with recent 429 status",
			channel: &Channel{
				ID:   1,
				Name: "test",
				Keys: []ChannelKey{
					{ID: 1, ChannelID: 1, Enabled: true, ChannelKey: "key1", StatusCode: 429, LastUseTimeStamp: now, TotalCost: 1.0},
					{ID: 2, ChannelID: 1, Enabled: true, ChannelKey: "key2", StatusCode: 200, TotalCost: 5.0},
				},
			},
			expectEmpty:   false,
			expectedKeyID: 2, // key1 is skipped due to recent 429
		},
		{
			name: "uses key with old 429 status (more than 5 minutes)",
			channel: &Channel{
				ID:   1,
				Name: "test",
				Keys: []ChannelKey{
					{ID: 1, ChannelID: 1, Enabled: true, ChannelKey: "key1", StatusCode: 429, LastUseTimeStamp: now - 400, TotalCost: 1.0}, // 400 seconds ago > 5 min
					{ID: 2, ChannelID: 1, Enabled: true, ChannelKey: "key2", StatusCode: 200, TotalCost: 5.0},
				},
			},
			expectEmpty:   false,
			expectedKeyID: 1, // key1's 429 is old, it has lower cost
		},
		{
			name: "custom filter excludes key",
			channel: &Channel{
				ID:   1,
				Name: "test",
				Keys: []ChannelKey{
					{ID: 1, ChannelID: 1, Enabled: true, ChannelKey: "key1", TotalCost: 1.0},
					{ID: 2, ChannelID: 1, Enabled: true, ChannelKey: "key2", TotalCost: 5.0},
				},
			},
			filters: []KeyFilter{
				func(k ChannelKey) bool { return k.ID != 1 }, // exclude key1
			},
			expectEmpty:   false,
			expectedKeyID: 2,
		},
		{
			name: "multiple filters work together",
			channel: &Channel{
				ID:   1,
				Name: "test",
				Keys: []ChannelKey{
					{ID: 1, ChannelID: 1, Enabled: true, ChannelKey: "key1", TotalCost: 1.0},
					{ID: 2, ChannelID: 1, Enabled: true, ChannelKey: "key2", TotalCost: 5.0},
					{ID: 3, ChannelID: 1, Enabled: true, ChannelKey: "key3", TotalCost: 8.0},
				},
			},
			filters: []KeyFilter{
				func(k ChannelKey) bool { return k.ID != 1 }, // exclude key1
				func(k ChannelKey) bool { return k.ID != 2 }, // exclude key2
			},
			expectEmpty:   false,
			expectedKeyID: 3,
		},
		{
			name: "all keys filtered returns empty",
			channel: &Channel{
				ID:   1,
				Name: "test",
				Keys: []ChannelKey{
					{ID: 1, ChannelID: 1, Enabled: true, ChannelKey: "key1", TotalCost: 1.0},
				},
			},
			filters: []KeyFilter{
				func(k ChannelKey) bool { return false }, // exclude all
			},
			expectEmpty: true,
		},
		{
			name: "simulates circuit breaker filter - all tripped",
			channel: &Channel{
				ID:   1,
				Name: "test",
				Keys: []ChannelKey{
					{ID: 1, ChannelID: 1, Enabled: true, ChannelKey: "key1", TotalCost: 1.0},
					{ID: 2, ChannelID: 1, Enabled: true, ChannelKey: "key2", TotalCost: 5.0},
				},
			},
			filters: []KeyFilter{
				func(k ChannelKey) bool { return false }, // simulate all keys tripped
			},
			expectEmpty: true,
		},
		{
			name: "simulates circuit breaker filter - one available",
			channel: &Channel{
				ID:   1,
				Name: "test",
				Keys: []ChannelKey{
					{ID: 1, ChannelID: 1, Enabled: true, ChannelKey: "key1", TotalCost: 1.0},
					{ID: 2, ChannelID: 1, Enabled: true, ChannelKey: "key2", TotalCost: 5.0},
				},
			},
			filters: []KeyFilter{
				func(k ChannelKey) bool { return k.ID == 2 }, // only key2 is not tripped
			},
			expectEmpty:   false,
			expectedKeyID: 2,
		},
		{
			name: "simulates tried keys filter",
			channel: &Channel{
				ID:   1,
				Name: "test",
				Keys: []ChannelKey{
					{ID: 1, ChannelID: 1, Enabled: true, ChannelKey: "key1", TotalCost: 1.0},
					{ID: 2, ChannelID: 1, Enabled: true, ChannelKey: "key2", TotalCost: 5.0},
					{ID: 3, ChannelID: 1, Enabled: true, ChannelKey: "key3", TotalCost: 8.0},
				},
			},
			filters: []KeyFilter{
				func(k ChannelKey) bool { return k.ID != 1 }, // key1 already tried (429)
			},
			expectEmpty:   false,
			expectedKeyID: 2, // key2 has lowest cost among remaining
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var key ChannelKey
			if tt.filters != nil {
				key = tt.channel.GetChannelKey(tt.filters...)
			} else {
				key = tt.channel.GetChannelKey()
			}

			if tt.expectEmpty {
				if key.ChannelKey != "" {
					t.Errorf("expected empty key, got key with ID %d", key.ID)
				}
			} else {
				if key.ChannelKey == "" {
					t.Error("expected non-empty key, got empty")
					return
				}
				if key.ID != tt.expectedKeyID {
					t.Errorf("expected key ID %d, got %d", tt.expectedKeyID, key.ID)
				}
			}
		})
	}
}

func TestChannel_GetBaseUrl(t *testing.T) {
	tests := []struct {
		name     string
		channel  *Channel
		expected string
	}{
		{
			name:     "nil channel returns empty",
			channel:  nil,
			expected: "",
		},
		{
			name:     "channel with no endpoints returns empty",
			channel:  &Channel{ID: 1},
			expected: "",
		},
		{
			name: "returns first enabled endpoint base_url",
			channel: &Channel{
				ID: 1,
				Endpoints: []Endpoint{
					{Type: outbound.OutboundTypeOpenAIChat, BaseUrl: "https://api1.example.com", Enabled: true},
					{Type: outbound.OutboundTypeAnthropic, BaseUrl: "https://api2.example.com", Enabled: true},
				},
			},
			expected: "https://api1.example.com",
		},
		{
			name: "skips disabled endpoints",
			channel: &Channel{
				ID: 1,
				Endpoints: []Endpoint{
					{Type: outbound.OutboundTypeOpenAIChat, BaseUrl: "https://api1.example.com", Enabled: false},
					{Type: outbound.OutboundTypeAnthropic, BaseUrl: "https://api2.example.com", Enabled: true},
				},
			},
			expected: "https://api2.example.com",
		},
		{
			name: "skips empty base_url",
			channel: &Channel{
				ID: 1,
				Endpoints: []Endpoint{
					{Type: outbound.OutboundTypeOpenAIChat, BaseUrl: "", Enabled: true},
					{Type: outbound.OutboundTypeAnthropic, BaseUrl: "https://api.example.com", Enabled: true},
				},
			},
			expected: "https://api.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.channel.GetBaseUrl()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestChannel_MatchEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		channel      *Channel
		inboundType  inbound.InboundType
		expectNil    bool
		expectExact  bool
		expectedType outbound.OutboundType
		expectedUrl  string
	}{
		{
			name:        "nil channel returns nil",
			channel:     nil,
			inboundType: inbound.InboundTypeOpenAIChat,
			expectNil:   true,
		},
		{
			name:        "no endpoints returns nil",
			channel:     &Channel{ID: 1},
			inboundType: inbound.InboundTypeOpenAIChat,
			expectNil:   true,
		},
		{
			name: "exact match OpenAI",
			channel: &Channel{
				ID: 1,
				Endpoints: []Endpoint{
					{Type: outbound.OutboundTypeOpenAIChat, BaseUrl: "https://api.openai.com/v1", Enabled: true},
					{Type: outbound.OutboundTypeAnthropic, BaseUrl: "https://api.anthropic.com/v1", Enabled: true},
				},
			},
			inboundType:  inbound.InboundTypeOpenAIChat,
			expectNil:    false,
			expectExact:  true,
			expectedType: outbound.OutboundTypeOpenAIChat,
			expectedUrl:  "https://api.openai.com/v1",
		},
		{
			name: "exact match Anthropic",
			channel: &Channel{
				ID: 1,
				Endpoints: []Endpoint{
					{Type: outbound.OutboundTypeOpenAIChat, BaseUrl: "https://api.openai.com/v1", Enabled: true},
					{Type: outbound.OutboundTypeAnthropic, BaseUrl: "https://api.anthropic.com/v1", Enabled: true},
				},
			},
			inboundType:  inbound.InboundTypeAnthropic,
			expectNil:    false,
			expectExact:  true,
			expectedType: outbound.OutboundTypeAnthropic,
			expectedUrl:  "https://api.anthropic.com/v1",
		},
		{
			name: "fallback when no exact match",
			channel: &Channel{
				ID: 1,
				Endpoints: []Endpoint{
					{Type: outbound.OutboundTypeOpenAIChat, BaseUrl: "https://api.openai.com/v1", Enabled: true},
				},
			},
			inboundType:  inbound.InboundTypeAnthropic,
			expectNil:    false,
			expectExact:  false,
			expectedType: outbound.OutboundTypeOpenAIChat,
			expectedUrl:  "https://api.openai.com/v1",
		},
		{
			name: "skips disabled endpoint for exact match, falls back",
			channel: &Channel{
				ID: 1,
				Endpoints: []Endpoint{
					{Type: outbound.OutboundTypeOpenAIChat, BaseUrl: "https://api.openai.com/v1", Enabled: false},
					{Type: outbound.OutboundTypeAnthropic, BaseUrl: "https://api.anthropic.com/v1", Enabled: true},
				},
			},
			inboundType:  inbound.InboundTypeOpenAIChat,
			expectNil:    false,
			expectExact:  false,
			expectedType: outbound.OutboundTypeAnthropic,
			expectedUrl:  "https://api.anthropic.com/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep, exact := tt.channel.MatchEndpoint(tt.inboundType)
			if tt.expectNil {
				if ep != nil {
					t.Error("expected nil endpoint")
				}
				return
			}
			if ep == nil {
				t.Fatal("expected non-nil endpoint")
			}
			if exact != tt.expectExact {
				t.Errorf("expected exact=%v, got exact=%v", tt.expectExact, exact)
			}
			if ep.Type != tt.expectedType {
				t.Errorf("expected type %d, got %d", tt.expectedType, ep.Type)
			}
			if ep.BaseUrl != tt.expectedUrl {
				t.Errorf("expected url %q, got %q", tt.expectedUrl, ep.BaseUrl)
			}
		})
	}
}
