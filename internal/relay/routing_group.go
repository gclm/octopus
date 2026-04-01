package relay

import (
	"context"
	"strings"

	dbmodel "github.com/gclm/octopus/internal/model"
	"github.com/gclm/octopus/internal/op"
)

type groupLookup func(name string, ctx context.Context) (dbmodel.Group, error)

var reasoningGroupSuffixes = []string{"-medium", "-high", "-xhigh"}
var supportedReasoningGroupEfforts = map[string]bool{
	"medium": true,
	"high":   true,
	"xhigh":  true,
}

func normalizeReasoningEffort(effort string) string {
	normalized := strings.ToLower(strings.TrimSpace(effort))
	switch normalized {
	case "extra high", "extra-high", "extra_high":
		return "xhigh"
	default:
		return normalized
	}
}

func hasReasoningGroupSuffix(model string) bool {
	lower := strings.ToLower(strings.TrimSpace(model))
	for _, suffix := range reasoningGroupSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func preferredReasoningGroupModel(requestModel, reasoningEffort string) string {
	effort := normalizeReasoningEffort(reasoningEffort)
	if requestModel == "" || effort == "" || hasReasoningGroupSuffix(requestModel) {
		return ""
	}
	if !supportedReasoningGroupEfforts[effort] {
		return ""
	}
	return requestModel + "-" + effort
}

func resolveRoutingGroup(requestModel, reasoningEffort, supportedModels string, ctx context.Context) (dbmodel.Group, string, error) {
	return resolveRoutingGroupWithLookup(requestModel, reasoningEffort, supportedModels, ctx, op.GroupGetEnabledMap)
}

func resolveRoutingGroupWithLookup(requestModel, reasoningEffort, supportedModels string, ctx context.Context, lookup groupLookup) (dbmodel.Group, string, error) {
	if preferredModel := preferredReasoningGroupModel(requestModel, reasoningEffort); preferredModel != "" {
		if isModelAllowed(supportedModels, preferredModel) {
			group, err := lookup(preferredModel, ctx)
			if err == nil {
				return group, preferredModel, nil
			}
		}
	}

	group, err := lookup(requestModel, ctx)
	if err != nil {
		return dbmodel.Group{}, "", err
	}
	return group, requestModel, nil
}

func isModelAllowed(supportedModels, model string) bool {
	if strings.TrimSpace(supportedModels) == "" {
		return true
	}
	for _, candidate := range strings.Split(supportedModels, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if candidate == model {
			return true
		}
	}
	return false
}
