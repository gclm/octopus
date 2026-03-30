package balancer

import (
	"crypto/x509"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/bestruirui/octopus/internal/model"
)

func prepareCircuitTest(t *testing.T) {
	t.Helper()
	globalBreaker = sync.Map{}
	globalSession = sync.Map{}
	testSettingIntOverride = func(key model.SettingKey) (int, bool) {
		switch key {
		case model.SettingKeyCircuitBreakerThreshold:
			return 5, true
		case model.SettingKeyCircuitBreakerCooldown:
			return 60, true
		case model.SettingKeyCircuitBreakerMaxCooldown:
			return 7200, true
		case model.SettingKeyCircuitBreakerHealthScoreThreshold:
			return -50, true
		case model.SettingKeyCircuitBreakerHealthScoreMin:
			return -100, true
		case model.SettingKeyCircuitBreakerHealthScoreMax:
			return 100, true
		case model.SettingKeyCircuitBreakerHealthScoreDecayStep:
			return 5, true
		case model.SettingKeyCircuitBreakerHealthScoreDecayIntervalSeconds:
			return 600, true
		case model.SettingKeyCircuitBreakerHealthScoreWarmupSuccesses:
			return 3, true
		default:
			return 0, false
		}
	}
	t.Cleanup(func() {
		globalBreaker = sync.Map{}
		globalSession = sync.Map{}
		testSettingIntOverride = nil
	})
}

func TestClassifyFailureTLS(t *testing.T) {
	prepareCircuitTest(t)
	kind := ClassifyFailure(fmt.Errorf("tls failed: %w", x509.UnknownAuthorityError{}), 0)
	if kind != FailureTLSError {
		t.Fatalf("expected tls error, got %s", kind)
	}
}

func TestRecordFailureTLSOpensImmediately(t *testing.T) {
	prepareCircuitTest(t)
	key := circuitKey(1, 1, "gpt-5.4")
	RecordFailure(1, 1, "gpt-5.4", FailureTLSError)
	v, ok := globalBreaker.Load(key)
	if !ok {
		t.Fatal("expected breaker entry")
	}
	entry := v.(*circuitEntry)
	if entry.State != StateOpen {
		t.Fatalf("expected open state, got %v", entry.State)
	}
	if entry.HealthScore != -50 {
		t.Fatalf("expected -50 health score, got %d", entry.HealthScore)
	}
	remaining := time.Until(entry.OpenUntil)
	if remaining < 25*time.Minute {
		t.Fatalf("expected long cooldown for tls error, got %v", remaining)
	}
}

func TestRecordFailureFirstTokenTimeoutPenalizesHealth(t *testing.T) {
	prepareCircuitTest(t)
	key := circuitKey(2, 2, "gpt-5.4")
	RecordFailure(2, 2, "gpt-5.4", FailureFirstTokenTimeout)
	v, ok := globalBreaker.Load(key)
	if !ok {
		t.Fatal("expected breaker entry")
	}
	entry := v.(*circuitEntry)
	if entry.HealthScore != -20 {
		t.Fatalf("expected -20 health score, got %d", entry.HealthScore)
	}
	if entry.State != StateClosed {
		t.Fatalf("expected closed state before threshold, got %v", entry.State)
	}
}

func TestRecordSuccessResetsStateAndImprovesHealth(t *testing.T) {
	prepareCircuitTest(t)
	key := circuitKey(3, 3, "gpt-5.4")
	RecordFailure(3, 3, "gpt-5.4", FailureNetworkTimeout)
	RecordSuccess(3, 3, "gpt-5.4", 500*time.Millisecond)
	v, ok := globalBreaker.Load(key)
	if !ok {
		t.Fatal("expected breaker entry")
	}
	entry := v.(*circuitEntry)
	if entry.State != StateClosed {
		t.Fatalf("expected closed state, got %v", entry.State)
	}
	if entry.TripCount != 0 {
		t.Fatalf("expected trip count reset, got %d", entry.TripCount)
	}
	if entry.HealthScore != -10 {
		t.Fatalf("expected health recovery to -10, got %d", entry.HealthScore)
	}
	if !entry.OpenUntil.IsZero() {
		t.Fatalf("expected open until reset, got %v", entry.OpenUntil)
	}
}

func TestClassifyFailureByStatusCode(t *testing.T) {
	prepareCircuitTest(t)
	if got := ClassifyFailure(nil, http.StatusUnauthorized); got != FailureAuthError {
		t.Fatalf("expected auth error, got %s", got)
	}
	if got := ClassifyFailure(nil, http.StatusNotFound); got != FailureModelNotFound {
		t.Fatalf("expected model not found, got %s", got)
	}
	if got := ClassifyFailure(nil, http.StatusBadGateway); got != FailureUpstream5xx {
		t.Fatalf("expected upstream 5xx, got %s", got)
	}
}

func TestHealthScoreDecayTowardZero(t *testing.T) {
	prepareCircuitTest(t)
	score := decayHealthScore(-30, time.Now().Add(-31*time.Minute), time.Now())
	if score != -15 {
		t.Fatalf("expected decayed score -15, got %d", score)
	}
	score = decayHealthScore(30, time.Now().Add(-31*time.Minute), time.Now())
	if score != 15 {
		t.Fatalf("expected decayed score 15, got %d", score)
	}
}

func TestAggregatedHealthScoreAppliesDecay(t *testing.T) {
	prepareCircuitTest(t)
	key := circuitKey(9, 1, "gpt-5.4")
	entry := getOrCreateEntry(key)
	entry.mu.Lock()
	entry.HealthScore = -30
	entry.LastFailureTime = time.Now().Add(-31 * time.Minute)
	entry.mu.Unlock()

	score := HealthScore(9, 0, "gpt-5.4")
	if score != -15 {
		t.Fatalf("expected aggregated decayed score -15, got %d", score)
	}
}

func TestResetChannelStateClearsBreakerAndSticky(t *testing.T) {
	prepareCircuitTest(t)
	RecordFailure(11, 1, "gpt-5.4", FailureNetworkTimeout)
	SetSticky(123, "gpt-5.4", 11, 1)

	ResetChannelState(11)

	if _, ok := globalBreaker.Load(circuitKey(11, 1, "gpt-5.4")); ok {
		t.Fatal("expected breaker entry to be removed")
	}
	if sticky := GetSticky(123, "gpt-5.4", time.Minute); sticky != nil {
		t.Fatal("expected sticky session to be removed")
	}
}
