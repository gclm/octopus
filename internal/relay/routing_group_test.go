package relay

import (
	"context"
	"fmt"
	"testing"

	dbmodel "github.com/gclm/octopus/internal/model"
)

func TestPreferredReasoningGroupModel(t *testing.T) {
	tests := []struct {
		name            string
		requestModel    string
		reasoningEffort string
		want            string
	}{
		{name: "medium", requestModel: "gpt-5.4", reasoningEffort: "medium", want: "gpt-5.4-medium"},
		{name: "high", requestModel: "gpt-5.4", reasoningEffort: "high", want: "gpt-5.4-high"},
		{name: "xhigh", requestModel: "gpt-5.4", reasoningEffort: "xhigh", want: "gpt-5.4-xhigh"},
		{name: "extra_high_alias", requestModel: "gpt-5.4", reasoningEffort: "extra high", want: "gpt-5.4-xhigh"},
		{name: "explicit_derived_group_kept", requestModel: "gpt-5.4-xhigh", reasoningEffort: "medium", want: ""},
		{name: "empty_effort", requestModel: "gpt-5.4", reasoningEffort: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := preferredReasoningGroupModel(tt.requestModel, tt.reasoningEffort); got != tt.want {
				t.Fatalf("preferredReasoningGroupModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveRoutingGroupWithLookup_PrefersDerivedGroup(t *testing.T) {
	lookup := func(name string, _ context.Context) (dbmodel.Group, error) {
		switch name {
		case "gpt-5.4-xhigh":
			return dbmodel.Group{Name: name}, nil
		case "gpt-5.4":
			return dbmodel.Group{Name: name}, nil
		default:
			return dbmodel.Group{}, fmt.Errorf("group not found")
		}
	}

	group, routingModel, err := resolveRoutingGroupWithLookup("gpt-5.4", "xhigh", context.Background(), lookup)
	if err != nil {
		t.Fatalf("resolveRoutingGroupWithLookup() error = %v", err)
	}
	if group.Name != "gpt-5.4-xhigh" || routingModel != "gpt-5.4-xhigh" {
		t.Fatalf("unexpected result: group=%q routing=%q", group.Name, routingModel)
	}
}

func TestResolveRoutingGroupWithLookup_FallsBackToBaseGroup(t *testing.T) {
	lookup := func(name string, _ context.Context) (dbmodel.Group, error) {
		switch name {
		case "gpt-5.4":
			return dbmodel.Group{Name: name}, nil
		default:
			return dbmodel.Group{}, fmt.Errorf("group not found")
		}
	}

	group, routingModel, err := resolveRoutingGroupWithLookup("gpt-5.4", "high", context.Background(), lookup)
	if err != nil {
		t.Fatalf("resolveRoutingGroupWithLookup() error = %v", err)
	}
	if group.Name != "gpt-5.4" || routingModel != "gpt-5.4" {
		t.Fatalf("unexpected result: group=%q routing=%q", group.Name, routingModel)
	}
}

func TestIsModelAllowed_AllowsBaseOrDerivedModel(t *testing.T) {
	if !isModelAllowed("gpt-5.4", "gpt-5.4", "gpt-5.4-xhigh") {
		t.Fatal("expected base model to be allowed")
	}
	if !isModelAllowed("gpt-5.4-xhigh", "gpt-5.4", "gpt-5.4-xhigh") {
		t.Fatal("expected derived model to be allowed")
	}
	if isModelAllowed("gpt-5.4-high", "gpt-5.4", "gpt-5.4-xhigh") {
		t.Fatal("unexpected unrelated model allowance")
	}
}
