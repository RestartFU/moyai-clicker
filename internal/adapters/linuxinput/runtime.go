package linuxinput

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"syscall"
	"time"

	"clicker/internal/core/autoclicker"

	evdev "github.com/holoplot/go-evdev"
)

type RuntimeConfig struct {
	TriggerCode  uint16
	ToggleCode   uint16
	CPS          float64
	ClickDown    time.Duration
	StartEnabled bool
	GrabDevices  bool
}

type Runtime struct {
	sourceDevices []*evdev.InputDevice
	grabPaths     map[string]struct{}
	grabEnabled   bool
	service       *autoclicker.Service
	logger        autoclicker.Logger

	stopCh    chan struct{}
	stopOnce  sync.Once
	readersWG sync.WaitGroup
}

type evdevInjector struct {
	dev *evdev.InputDevice
}

func (e *evdevInjector) WriteEvents(events ...autoclicker.Event) error {
	for _, event := range events {
		ev := evdev.InputEvent{
			Type:  evdev.EvType(event.Type),
			Code:  evdev.EvCode(event.Code),
			Value: event.Value,
		}
		if err := e.dev.WriteOne(&ev); err != nil {
			return err
		}
	}
	return nil
}

func (e *evdevInjector) Close() error {
	if e.dev == nil {
		return nil
	}
	return e.dev.Close()
}

func NewRuntime(selection *SourceSelection, cfg RuntimeConfig, logger autoclicker.Logger) (*Runtime, error) {
	if selection == nil {
		return nil, fmt.Errorf("source selection is nil")
	}
	if len(selection.Devices) == 0 {
		return nil, fmt.Errorf("source selection has no devices")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}

	grabPaths := make(map[string]struct{}, len(selection.TriggerPaths))
	grabEnabled := false
	if cfg.GrabDevices {
		for _, dev := range selection.Devices {
			path := dev.Path()
			if _, ok := selection.TriggerPaths[path]; !ok {
				continue
			}
			if deviceHasRelativeXY(dev) {
				grabPaths[path] = struct{}{}
				continue
			}
			name, _ := dev.Name()
			logger.Warn(
				"Not grabbing source device; absolute-motion passthrough is unreliable, using non-grab",
				"path", path,
				"name", name,
			)
		}
		if len(grabPaths) > 0 {
			grabEnabled = true
		} else {
			logger.Warn("No grab-capable trigger devices detected; running in non-grab mode")
		}
	}

	capabilities := buildUinputCapabilities(selection.Devices, grabPaths, cfg.TriggerCode, cfg.ToggleCode, grabEnabled)
	id := evdev.InputID{
		BusType: uint16(evdev.BUS_VIRTUAL),
		Vendor:  0x1,
		Product: 0x1,
		Version: 1,
	}
	if sourceID, err := selection.Devices[0].InputID(); err == nil {
		id = sourceID
		id.BusType = uint16(evdev.BUS_VIRTUAL)
	}

	injectorDev, err := evdev.CreateDevice("hold-autoclicker", id, capabilities)
	if err != nil {
		return nil, err
	}
	injector := &evdevInjector{dev: injectorDev}

	service, err := autoclicker.NewService(
		autoclicker.Config{
			TriggerCode:    cfg.TriggerCode,
			ToggleCode:     cfg.ToggleCode,
			TriggerSources: selection.TriggerPaths,
			ToggleSources:  selection.TogglePaths,
			GrabSources:    grabPaths,
			GrabEnabled:    grabEnabled,
			CPS:            cfg.CPS,
			ClickDown:      cfg.ClickDown,
			StartEnabled:   cfg.StartEnabled,
		},
		injector,
		logger,
	)
	if err != nil {
		_ = injector.Close()
		return nil, err
	}

	return &Runtime{
		sourceDevices: selection.Devices,
		grabPaths:     grabPaths,
		grabEnabled:   grabEnabled,
		service:       service,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}, nil
}

func (r *Runtime) Start() error {
	grabbed := make([]*evdev.InputDevice, 0, len(r.sourceDevices))
	if r.grabEnabled {
		for _, dev := range r.sourceDevices {
			if _, ok := r.grabPaths[dev.Path()]; !ok {
				continue
			}
			if err := dev.Grab(); err != nil {
				for _, device := range grabbed {
					_ = device.Ungrab()
				}
				return err
			}
			grabbed = append(grabbed, dev)
			name, _ := dev.Name()
			r.logger.Info("Grabbed source device", "path", dev.Path(), "name", name)
		}
	}

	for _, dev := range r.sourceDevices {
		if err := dev.NonBlock(); err != nil {
			return fmt.Errorf("failed to set nonblocking mode for %s: %w", dev.Path(), err)
		}
	}

	r.service.Start()
	for _, dev := range r.sourceDevices {
		r.readersWG.Add(1)
		go r.readLoop(dev)
	}
	return nil
}

func (r *Runtime) Stop() {
	r.stopOnce.Do(func() {
		close(r.stopCh)
		if r.grabEnabled {
			for _, dev := range r.sourceDevices {
				if _, ok := r.grabPaths[dev.Path()]; !ok {
					continue
				}
				_ = dev.Ungrab()
			}
		}
		for _, dev := range r.sourceDevices {
			_ = dev.Close()
		}
		r.readersWG.Wait()
		r.service.Stop()
	})
}

func (r *Runtime) SetEnabled(enabled bool) {
	r.service.SetEnabled(enabled)
}

func (r *Runtime) IsEnabled() bool {
	return r.service.IsEnabled()
}

func (r *Runtime) SetCPS(cps float64) error {
	return r.service.SetCPS(cps)
}

func (r *Runtime) GrabEnabled() bool {
	return r.grabEnabled
}

func (r *Runtime) readLoop(dev *evdev.InputDevice) {
	defer r.readersWG.Done()

	path := dev.Path()
	for {
		events, err := dev.ReadSlice(64)
		if err != nil {
			if r.stopped() || isDeviceClosedError(err) {
				return
			}
			if isWouldBlockError(err) {
				if !r.sleepWithStop(10 * time.Millisecond) {
					return
				}
				continue
			}
			r.logger.Warn("Read failed", "path", path, "err", err)
			if !r.sleepWithStop(100 * time.Millisecond) {
				return
			}
			continue
		}

		for _, event := range events {
			if !r.service.SubmitEvent(path, autoclicker.Event{
				Type:  uint16(event.Type),
				Code:  uint16(event.Code),
				Value: event.Value,
			}) {
				return
			}
		}
	}
}

func (r *Runtime) stopped() bool {
	select {
	case <-r.stopCh:
		return true
	default:
		return false
	}
}

func (r *Runtime) sleepWithStop(duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-r.stopCh:
		return false
	case <-timer.C:
		return true
	}
}

func buildUinputCapabilities(
	sourceDevices []*evdev.InputDevice,
	grabPaths map[string]struct{},
	triggerCode uint16,
	toggleCode uint16,
	grabEnabled bool,
) map[evdev.EvType][]evdev.EvCode {
	keyCodes := map[evdev.EvCode]struct{}{evdev.BTN_LEFT: {}}
	relCodes := make(map[evdev.EvCode]struct{})

	if grabEnabled {
		for _, dev := range sourceDevices {
			if _, ok := grabPaths[dev.Path()]; !ok {
				continue
			}
			for _, code := range dev.CapableEvents(evdev.EV_KEY) {
				if uint16(code) == triggerCode && triggerCode != uint16(evdev.BTN_LEFT) {
					continue
				}
				if uint16(code) == toggleCode {
					continue
				}
				keyCodes[code] = struct{}{}
			}
			for _, code := range dev.CapableEvents(evdev.EV_REL) {
				relCodes[code] = struct{}{}
			}
		}
	}

	capabilities := map[evdev.EvType][]evdev.EvCode{
		evdev.EV_KEY: sortedCodes(keyCodes),
	}
	if len(relCodes) > 0 {
		capabilities[evdev.EV_REL] = sortedCodes(relCodes)
	}
	return capabilities
}

func sortedCodes(values map[evdev.EvCode]struct{}) []evdev.EvCode {
	codes := make([]evdev.EvCode, 0, len(values))
	for code := range values {
		codes = append(codes, code)
	}
	sort.Slice(codes, func(i, j int) bool {
		return codes[i] < codes[j]
	})
	return codes
}

func isDeviceClosedError(err error) bool {
	return errors.Is(err, syscall.EBADF) || errors.Is(err, syscall.ENODEV)
}

func isWouldBlockError(err error) bool {
	return errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK)
}
