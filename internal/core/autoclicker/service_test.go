package autoclicker

import (
	"sync"
	"testing"
	"time"
)

type recordingInjector struct {
	mu     sync.Mutex
	events []Event
	closed bool
}

func (r *recordingInjector) WriteEvents(events ...Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, events...)
	return nil
}

func (r *recordingInjector) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

func (r *recordingInjector) snapshot() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Event, len(r.events))
	copy(out, r.events)
	return out
}

func (r *recordingInjector) isClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

type noopLogger struct{}

func (noopLogger) Debug(string, ...any) {}
func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}

func testConfig(startEnabled bool) Config {
	return Config{
		TriggerCode:    LeftButtonCode,
		ToggleCode:     LeftButtonCode + 1,
		TriggerSources: map[string]struct{}{"device": {}},
		ToggleSources:  map[string]struct{}{"device": {}},
		GrabSources:    map[string]struct{}{},
		GrabEnabled:    false,
		CPS:            10,
		StartEnabled:   startEnabled,
	}
}

func assertReleaseSuffix(t *testing.T, events []Event) {
	t.Helper()
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	up := events[len(events)-2]
	syn := events[len(events)-1]
	if up != (Event{Type: EventTypeKey, Code: LeftButtonCode, Value: 0}) {
		t.Fatalf("unexpected release event: %#v", up)
	}
	if syn != (Event{Type: EventTypeSyn, Code: SynReportCode, Value: 0}) {
		t.Fatalf("unexpected sync event: %#v", syn)
	}
}

func TestSetEnabledFalseReleasesLeftButton(t *testing.T) {
	injector := &recordingInjector{}
	service, err := NewService(testConfig(true), injector, noopLogger{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.writeEvents(
		Event{Type: EventTypeKey, Code: LeftButtonCode, Value: 1},
		Event{Type: EventTypeSyn, Code: SynReportCode, Value: 0},
	); err != nil {
		t.Fatalf("writeEvents() error = %v", err)
	}
	if !service.leftButtonDown.Load() {
		t.Fatalf("expected left button to be tracked as down")
	}

	service.SetEnabled(false)

	if service.leftButtonDown.Load() {
		t.Fatalf("expected left button to be tracked as up after disabling")
	}
	assertReleaseSuffix(t, injector.snapshot())
}

func TestSetEnabledTrueReleasesStaleLeftButton(t *testing.T) {
	injector := &recordingInjector{}
	service, err := NewService(testConfig(false), injector, noopLogger{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.writeEvents(
		Event{Type: EventTypeKey, Code: LeftButtonCode, Value: 1},
		Event{Type: EventTypeSyn, Code: SynReportCode, Value: 0},
	); err != nil {
		t.Fatalf("writeEvents() error = %v", err)
	}
	if !service.leftButtonDown.Load() {
		t.Fatalf("expected left button to be tracked as down")
	}

	service.SetEnabled(true)

	if service.leftButtonDown.Load() {
		t.Fatalf("expected left button to be tracked as up after enabling")
	}
	assertReleaseSuffix(t, injector.snapshot())
}

func TestStopReleasesLeftButtonBeforeClosingInjector(t *testing.T) {
	injector := &recordingInjector{}
	service, err := NewService(testConfig(true), injector, noopLogger{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.writeEvents(
		Event{Type: EventTypeKey, Code: LeftButtonCode, Value: 1},
		Event{Type: EventTypeSyn, Code: SynReportCode, Value: 0},
	); err != nil {
		t.Fatalf("writeEvents() error = %v", err)
	}

	service.Stop()

	if !injector.isClosed() {
		t.Fatalf("expected injector to be closed")
	}
	assertReleaseSuffix(t, injector.snapshot())
}

func TestHandleTriggerEventSignalsWakeOnFirstPressOnly(t *testing.T) {
	cfg := testConfig(true)
	cfg.TriggerCode = LeftButtonCode + 2
	cfg.ToggleCode = cfg.TriggerCode + 1

	service, err := NewService(cfg, &recordingInjector{}, noopLogger{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	service.handleTriggerEvent("device", 1)
	select {
	case <-service.wakeCh:
	default:
		t.Fatalf("expected wake signal on first trigger press")
	}

	service.handleTriggerEvent("device", 2)
	select {
	case <-service.wakeCh:
		t.Fatalf("expected no wake signal for repeat press while already holding")
	default:
	}

	service.handleTriggerEvent("device", 0)
	service.handleTriggerEvent("device", 1)
	select {
	case <-service.wakeCh:
	default:
		t.Fatalf("expected wake signal after trigger press transition")
	}
}

func TestWaitWithWakeReturnsOnSignal(t *testing.T) {
	cfg := testConfig(true)
	cfg.TriggerCode = LeftButtonCode + 2
	cfg.ToggleCode = cfg.TriggerCode + 1

	service, err := NewService(cfg, &recordingInjector{}, noopLogger{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	done := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		if !service.waitWithWake(5 * time.Second) {
			done <- -1
			return
		}
		done <- time.Since(start)
	}()

	time.Sleep(20 * time.Millisecond)
	service.signalWake()

	select {
	case elapsed := <-done:
		if elapsed < 0 {
			t.Fatalf("waitWithWake returned false")
		}
		if elapsed > 150*time.Millisecond {
			t.Fatalf("waitWithWake did not wake promptly: %v", elapsed)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("timeout waiting for wake")
	}
}

func TestSetToggleCodeAppliesToKnownSources(t *testing.T) {
	cfg := testConfig(true)
	cfg.TriggerCode = LeftButtonCode + 2
	cfg.ToggleCode = cfg.TriggerCode + 1
	cfg.TriggerSources = map[string]struct{}{"trigger-device": {}}
	cfg.ToggleSources = map[string]struct{}{"toggle-device": {}}

	service, err := NewService(cfg, &recordingInjector{}, noopLogger{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	newToggle := cfg.ToggleCode + 5

	service.handleEvent("trigger-device", Event{Type: EventTypeKey, Code: newToggle, Value: 1})
	if !service.IsEnabled() {
		t.Fatalf("unexpected toggle before SetToggleCode")
	}

	service.SetToggleCode(newToggle)

	service.handleEvent("trigger-device", Event{Type: EventTypeKey, Code: newToggle, Value: 1})
	if service.IsEnabled() {
		t.Fatalf("expected toggle after SetToggleCode on known source")
	}
}

func TestSetTriggerCodeSwitchesHandledTrigger(t *testing.T) {
	cfg := testConfig(true)
	cfg.TriggerCode = LeftButtonCode + 2
	cfg.ToggleCode = cfg.TriggerCode + 1

	service, err := NewService(cfg, &recordingInjector{}, noopLogger{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	service.handleEvent("device", Event{Type: EventTypeKey, Code: cfg.TriggerCode, Value: 1})
	if !service.holding.Load() {
		t.Fatalf("expected holding after initial trigger press")
	}

	newTrigger := cfg.TriggerCode + 5
	service.SetTriggerCode(newTrigger)
	if service.holding.Load() {
		t.Fatalf("expected holding cleared after SetTriggerCode")
	}

	service.handleEvent("device", Event{Type: EventTypeKey, Code: cfg.TriggerCode, Value: 1})
	if service.holding.Load() {
		t.Fatalf("old trigger code should no longer activate holding")
	}

	service.handleEvent("device", Event{Type: EventTypeKey, Code: newTrigger, Value: 1})
	if !service.holding.Load() {
		t.Fatalf("new trigger code should activate holding")
	}
}

func TestSetJitterRejectsNegative(t *testing.T) {
	service, err := NewService(testConfig(true), &recordingInjector{}, noopLogger{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.SetJitter(-1); err == nil {
		t.Fatalf("expected error when setting negative jitter")
	}
	if err := service.SetJitter(3); err != nil {
		t.Fatalf("SetJitter() error = %v", err)
	}
	if got := service.currentJitterPixels(); got != 3 {
		t.Fatalf("currentJitterPixels() = %d, want 3", got)
	}
}

func TestClickOnceEmitsAndRestoresJitterMotion(t *testing.T) {
	cfg := testConfig(true)
	cfg.CPS = 100
	cfg.ClickDown = 0
	cfg.JitterPixels = 3

	injector := &recordingInjector{}
	service, err := NewService(cfg, injector, noopLogger{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	for i := 0; i < 250; i++ {
		if ok := service.clickOnce(); !ok {
			t.Fatalf("clickOnce() returned false at iteration %d", i)
		}
	}

	var (
		relCount int
		sumX     int32
		sumY     int32
	)
	for _, event := range injector.snapshot() {
		if event.Type != EventTypeRel {
			continue
		}
		relCount++
		switch event.Code {
		case RelXCode:
			sumX += event.Value
		case RelYCode:
			sumY += event.Value
		}
	}

	if relCount == 0 {
		t.Fatalf("expected jitter to emit relative movement events")
	}
	if sumX != 0 || sumY != 0 {
		t.Fatalf("expected jitter movement to return to origin, got sumX=%d sumY=%d", sumX, sumY)
	}
}

func TestGrabTriggerPassesThroughWhenConfigured(t *testing.T) {
	cfg := testConfig(true)
	cfg.GrabEnabled = true
	cfg.GrabSources = map[string]struct{}{"device": {}}
	cfg.PassThroughTrigger = true

	injector := &recordingInjector{}
	service, err := NewService(cfg, injector, noopLogger{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	service.handleEvent("device", Event{Type: EventTypeKey, Code: cfg.TriggerCode, Value: 1})
	events := injector.snapshot()
	if len(events) == 0 {
		t.Fatalf("expected trigger down to be passed through when PassThroughTrigger is enabled")
	}
}
