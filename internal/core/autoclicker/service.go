package autoclicker

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type sourcedEvent struct {
	source string
	event  Event
}

type Service struct {
	cfg      Config
	injector Injector
	logger   Logger

	injectorMu sync.Mutex
	stateMu    sync.Mutex

	intervalNanos  atomic.Int64
	triggerCode    atomic.Uint32
	toggleCode     atomic.Uint32
	clickCount     atomic.Int64
	enabled        atomic.Bool
	holding        atomic.Bool
	leftButtonDown atomic.Bool

	pressedSources map[string]struct{}
	eventsCh       chan sourcedEvent
	wakeCh         chan struct{}
	stopCh         chan struct{}
	stopOnce       sync.Once
	workersWG      sync.WaitGroup
}

func NewService(cfg Config, injector Injector, logger Logger) (*Service, error) {
	if cfg.CPS <= 0 {
		return nil, fmt.Errorf("cps must be > 0")
	}
	if len(cfg.TriggerSources) == 0 {
		return nil, fmt.Errorf("no trigger-capable source devices configured")
	}
	if injector == nil {
		return nil, fmt.Errorf("injector is nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}

	service := &Service{
		cfg:            cfg,
		injector:       injector,
		logger:         logger,
		pressedSources: make(map[string]struct{}),
		eventsCh:       make(chan sourcedEvent, 256),
		wakeCh:         make(chan struct{}, 1),
		stopCh:         make(chan struct{}),
	}
	service.intervalNanos.Store(time.Duration(float64(time.Second) / cfg.CPS).Nanoseconds())
	service.triggerCode.Store(uint32(cfg.TriggerCode))
	service.toggleCode.Store(uint32(cfg.ToggleCode))
	service.enabled.Store(cfg.StartEnabled)
	return service, nil
}

func (s *Service) Start() {
	s.workersWG.Add(1)
	go s.eventLoop()

	s.workersWG.Add(1)
	go s.clickLoop()
}

func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		s.workersWG.Wait()
		s.releaseLeftButton()
		_ = s.injector.Close()
	})
}

func (s *Service) SubmitEvent(source string, event Event) bool {
	select {
	case <-s.stopCh:
		return false
	case s.eventsCh <- sourcedEvent{source: source, event: event}:
		return true
	}
}

func (s *Service) SetCPS(cps float64) error {
	if cps <= 0 {
		return fmt.Errorf("cps must be > 0")
	}
	s.intervalNanos.Store(time.Duration(float64(time.Second) / cps).Nanoseconds())
	return nil
}

func (s *Service) SetEnabled(enabled bool) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if s.enabled.Load() == enabled {
		// Defensive release in case button-up was missed.
		if enabled {
			s.releaseLeftButton()
		}
		return
	}
	s.enabled.Store(enabled)
	s.holding.Store(false)
	clear(s.pressedSources)
	s.releaseLeftButton()
	if !enabled {
		s.logger.Info("Autoclicker disabled")
		return
	}
	s.logger.Info("Autoclicker enabled")
}

func (s *Service) SetToggleCode(code uint16) {
	s.toggleCode.Store(uint32(code))
}

func (s *Service) SetTriggerCode(code uint16) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	s.triggerCode.Store(uint32(code))
	s.holding.Store(false)
	clear(s.pressedSources)
	s.releaseLeftButton()
}

func (s *Service) IsEnabled() bool {
	return s.enabled.Load()
}

func (s *Service) clickLoop() {
	defer s.workersWG.Done()

	lastProgress := time.Now()
	for {
		if s.stopped() {
			return
		}
		if !s.enabled.Load() || !s.holding.Load() {
			if !s.waitForWake() {
				return
			}
			continue
		}

		cycleStart := time.Now()
		if !s.clickOnce() {
			return
		}

		now := time.Now()
		if now.Sub(lastProgress) >= time.Second {
			s.logger.Info("Clicks sent", "count", s.clickCount.Load())
			lastProgress = now
		}

		interval := s.currentInterval()
		sleepFor := interval - time.Since(cycleStart)
		if sleepFor > 0 && !s.waitWithWake(sleepFor) {
			return
		}
	}
}

func (s *Service) eventLoop() {
	defer s.workersWG.Done()

	for {
		select {
		case <-s.stopCh:
			return
		case item := <-s.eventsCh:
			s.handleEvent(item.source, item.event)
		}
	}
}

func (s *Service) handleEvent(source string, event Event) {
	if event.Type == EventTypeKey && event.Code == s.currentToggleCode() && s.isKnownSource(source) {
		if event.Value == 1 {
			s.SetEnabled(!s.enabled.Load())
		}
		return
	}

	if event.Type == EventTypeKey && event.Code == s.currentTriggerCode() && s.isTriggerSource(source) {
		if !s.enabled.Load() && s.cfg.GrabEnabled && s.isGrabSource(source) {
			s.passThroughEvent(event)
			return
		}
		s.handleTriggerEvent(source, event.Value)
		return
	}

	if s.cfg.GrabEnabled && s.isGrabSource(source) {
		s.passThroughEvent(event)
	}
}

func (s *Service) handleTriggerEvent(source string, value int32) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	switch value {
	case 1, 2:
		if !s.enabled.Load() {
			return
		}
		wasHolding := len(s.pressedSources) > 0
		if _, exists := s.pressedSources[source]; !exists {
			s.logger.Info("Trigger down", "source", source)
		}
		s.pressedSources[source] = struct{}{}
		s.holding.Store(true)
		if !wasHolding {
			s.signalWake()
		}
		s.maybeNeutralizeLeftHold()
	case 0:
		if _, exists := s.pressedSources[source]; exists {
			s.logger.Info("Trigger up", "source", source)
		}
		delete(s.pressedSources, source)
		if len(s.pressedSources) == 0 {
			s.holding.Store(false)
		}
	}
}

func (s *Service) maybeNeutralizeLeftHold() {
	if s.cfg.GrabEnabled || s.currentTriggerCode() != LeftButtonCode {
		return
	}
	_ = s.writeEvents(
		Event{Type: EventTypeKey, Code: LeftButtonCode, Value: 0},
		Event{Type: EventTypeSyn, Code: SynReportCode, Value: 0},
	)
}

func (s *Service) passThroughEvent(event Event) {
	switch event.Type {
	case EventTypeKey, EventTypeRel:
		_ = s.writeEvents(event)
	case EventTypeSyn:
		if event.Code == SynReportCode {
			_ = s.writeEvents(Event{Type: EventTypeSyn, Code: SynReportCode, Value: 0})
		}
	}
}

func (s *Service) clickOnce() bool {
	interval := s.currentInterval()
	err := s.writeEvents(
		Event{Type: EventTypeKey, Code: LeftButtonCode, Value: 1},
		Event{Type: EventTypeSyn, Code: SynReportCode, Value: 0},
	)
	if err != nil {
		if s.stopped() {
			return false
		}
		s.logger.Warn("Failed to emit click down", "err", err)
		if !s.sleepWithStop(100 * time.Millisecond) {
			return false
		}
		return true
	}

	down := s.cfg.ClickDown
	if down > interval {
		down = interval
	}
	if down > 0 && !s.sleepWithStop(down) {
		return false
	}

	err = s.writeEvents(
		Event{Type: EventTypeKey, Code: LeftButtonCode, Value: 0},
		Event{Type: EventTypeSyn, Code: SynReportCode, Value: 0},
	)
	if err != nil {
		if s.stopped() {
			return false
		}
		s.logger.Warn("Failed to emit click up", "err", err)
		if !s.sleepWithStop(100 * time.Millisecond) {
			return false
		}
		return true
	}

	s.clickCount.Add(1)
	return true
}

func (s *Service) writeEvents(events ...Event) error {
	s.injectorMu.Lock()
	defer s.injectorMu.Unlock()
	if err := s.injector.WriteEvents(events...); err != nil {
		return err
	}
	s.trackLeftButtonState(events)
	return nil
}

func (s *Service) isTriggerSource(source string) bool {
	_, ok := s.cfg.TriggerSources[source]
	return ok
}

func (s *Service) isToggleSource(source string) bool {
	_, ok := s.cfg.ToggleSources[source]
	return ok
}

func (s *Service) isKnownSource(source string) bool {
	if s.isTriggerSource(source) {
		return true
	}
	return s.isToggleSource(source)
}

func (s *Service) isGrabSource(source string) bool {
	_, ok := s.cfg.GrabSources[source]
	return ok
}

func (s *Service) currentInterval() time.Duration {
	ns := s.intervalNanos.Load()
	if ns <= 0 {
		return time.Second
	}
	return time.Duration(ns)
}

func (s *Service) currentToggleCode() uint16 {
	return uint16(s.toggleCode.Load())
}

func (s *Service) currentTriggerCode() uint16 {
	return uint16(s.triggerCode.Load())
}

func (s *Service) stopped() bool {
	select {
	case <-s.stopCh:
		return true
	default:
		return false
	}
}

func (s *Service) signalWake() {
	select {
	case s.wakeCh <- struct{}{}:
	default:
	}
}

func (s *Service) waitForWake() bool {
	select {
	case <-s.stopCh:
		return false
	case <-s.wakeCh:
		return true
	}
}

func (s *Service) waitWithWake(duration time.Duration) bool {
	if duration <= 0 {
		return true
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-s.stopCh:
		return false
	case <-s.wakeCh:
		return true
	case <-timer.C:
		return true
	}
}

func (s *Service) sleepWithStop(duration time.Duration) bool {
	if duration <= 0 {
		return true
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-s.stopCh:
		return false
	case <-timer.C:
		return true
	}
}

func (s *Service) trackLeftButtonState(events []Event) {
	for _, event := range events {
		if event.Type != EventTypeKey || event.Code != LeftButtonCode {
			continue
		}
		switch event.Value {
		case 0:
			s.leftButtonDown.Store(false)
		case 1, 2:
			s.leftButtonDown.Store(true)
		}
	}
}

func (s *Service) releaseLeftButton() {
	if !s.leftButtonDown.Load() {
		return
	}
	if err := s.writeEvents(
		Event{Type: EventTypeKey, Code: LeftButtonCode, Value: 0},
		Event{Type: EventTypeSyn, Code: SynReportCode, Value: 0},
	); err != nil {
		s.logger.Warn("Failed to release left button", "err", err)
	}
}
