//go:build linux

package main

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strings"
	"syscall"
	"time"

	"clicker/internal/adapters/linuxinput"
	"clicker/internal/adapters/x11input"
)

func parseTriggerCode(value string) (uint16, error) {
	return linuxinput.ParseCode(value)
}

func parseBackendChoice(value string) (string, error) {
	backend := strings.ToLower(strings.TrimSpace(value))
	if backend == "" {
		backend = "auto"
	}
	switch backend {
	case "auto", "wayland", "x11", "evdev":
		return backend, nil
	default:
		return "", fmt.Errorf("invalid --backend %q (linux supports auto|wayland|x11)", value)
	}
}

func captureNextCode(backend, devicePath string, timeout time.Duration) (uint16, error) {
	switch resolveLinuxBackend(backend) {
	case "x11":
		return x11input.CaptureNextKeyCode(timeout)
	default:
		return linuxinput.CaptureNextKeyCode(devicePath, timeout)
	}
}

func formatCodeName(code uint16) string {
	return linuxinput.FormatCodeName(code)
}

func listInputDevices(backend string) error {
	switch resolveLinuxBackend(backend) {
	case "x11":
		devices, err := x11input.ListInputDevices()
		if err != nil {
			return err
		}
		for _, dev := range devices {
			virtualTag := "physical"
			if dev.IsVirtual {
				virtualTag = "virtual"
			}
			pointerTag := "non-pointer"
			if dev.IsPointer {
				pointerTag = "pointer"
			}
			fmt.Printf("%s: %s [%s, %s]\n", dev.Path, dev.Name, virtualTag, pointerTag)
		}
		return nil
	default:
		devices, err := linuxinput.ListInputDevices()
		if err != nil {
			return err
		}
		for _, dev := range devices {
			virtualTag := "physical"
			if dev.IsVirtual {
				virtualTag = "virtual"
			}
			pointerTag := "non-pointer"
			if dev.IsPointer {
				pointerTag = "pointer"
			}
			fmt.Printf("%s: %s [%s, %s]\n", dev.Path, dev.Name, virtualTag, pointerTag)
		}
		return nil
	}
}

func defaultGrabForTrigger(triggerCode uint16) bool {
	return triggerCode == linuxinput.CodeBTNLeft
}

func permissionDeniedHint() string {
	return "Permission denied opening input backend. On Wayland use root/udev for /dev/input + /dev/uinput. On X11 ensure an active X11 session and DISPLAY is set."
}

func startClickerFromConfig(cfg config, logger *slog.Logger) (clickerRuntime, error) {
	switch resolveLinuxBackend(cfg.backend) {
	case "x11":
		return startX11ClickerFromConfig(cfg, logger)
	default:
		return startWaylandClickerFromConfig(cfg, logger)
	}
}

func startWaylandClickerFromConfig(cfg config, logger *slog.Logger) (clickerRuntime, error) {
	return startWaylandClickerFromConfigWithRetry(cfg, logger, true)
}

func startWaylandClickerFromConfigWithRetry(cfg config, logger *slog.Logger, allowNoGrabFallback bool) (clickerRuntime, error) {
	selection, err := linuxinput.OpenSourceSelection(cfg.devicePath, cfg.triggerCode, cfg.toggleCode)
	if err != nil {
		return nil, err
	}

	for _, dev := range selection.Devices {
		name, _ := dev.Name()
		logger.Info("Using source device", "path", dev.Path(), "name", name)
	}

	clickDown := time.Duration(math.Max(0, cfg.downMS) * float64(time.Millisecond))
	runtime, err := linuxinput.NewRuntime(
		selection,
		linuxinput.RuntimeConfig{
			TriggerCode:  cfg.triggerCode,
			ToggleCode:   cfg.toggleCode,
			CPS:          cfg.cps,
			ClickDown:    clickDown,
			StartEnabled: cfg.startEnabled,
			GrabDevices:  cfg.grabDevices,
		},
		logger,
	)
	if err != nil {
		for _, dev := range selection.Devices {
			_ = dev.Close()
		}
		return nil, err
	}

	if err := runtime.Start(); err != nil {
		runtime.Stop()
		if allowNoGrabFallback && cfg.grabDevices && errors.Is(err, syscall.EBUSY) {
			logger.Warn("Grab failed (device busy), retrying without grab", "err", err)
			cfg.grabDevices = false
			return startWaylandClickerFromConfigWithRetry(cfg, logger, false)
		}
		return nil, err
	}

	logger.Info("Backend", "name", "wayland")
	logger.Info("Trigger", "name", formatCodeName(cfg.triggerCode), "code", cfg.triggerCode)
	logger.Info("Toggle", "name", formatCodeName(cfg.toggleCode), "code", cfg.toggleCode)
	logger.Info("Rate", "cps", cfg.cps)
	if runtime.GrabEnabled() {
		logger.Info("Grab mode enabled")
	} else {
		logger.Info("Grab mode disabled")
	}
	if cfg.triggerCode == linuxinput.CodeBTNLeft && !runtime.GrabEnabled() {
		logger.Warn("BTN_LEFT trigger without grabbing may be ignored by Wayland; use --grab")
	}
	if cfg.startEnabled {
		logger.Info("Initial state enabled (press toggle to disable/enable)")
	} else {
		logger.Info("Initial state disabled (press toggle to enable/disable)")
	}
	logger.Info("Hold trigger to autoclick left mouse button. Press Ctrl+C to stop")
	return runtime, nil
}

func startX11ClickerFromConfig(cfg config, logger *slog.Logger) (clickerRuntime, error) {
	if cfg.devicePath != "" {
		logger.Warn("--device is ignored on X11 backend")
	}
	if cfg.grabDevices {
		logger.Warn("--grab is ignored on X11 backend")
	}

	clickDown := time.Duration(math.Max(0, cfg.downMS) * float64(time.Millisecond))
	runtime, err := x11input.NewRuntime(
		x11input.RuntimeConfig{
			TriggerCode:  cfg.triggerCode,
			ToggleCode:   cfg.toggleCode,
			CPS:          cfg.cps,
			ClickDown:    clickDown,
			StartEnabled: cfg.startEnabled,
		},
		logger,
	)
	if err != nil {
		return nil, err
	}

	if err := runtime.Start(); err != nil {
		runtime.Stop()
		return nil, err
	}

	logger.Info("Backend", "name", "x11")
	logger.Info("Trigger", "name", formatCodeName(cfg.triggerCode), "code", cfg.triggerCode)
	logger.Info("Toggle", "name", formatCodeName(cfg.toggleCode), "code", cfg.toggleCode)
	logger.Info("Rate", "cps", cfg.cps)
	if cfg.startEnabled {
		logger.Info("Initial state enabled (press toggle to disable/enable)")
	} else {
		logger.Info("Initial state disabled (press toggle to enable/disable)")
	}
	logger.Info("Hold trigger to autoclick left mouse button. Press Ctrl+C to stop")
	return runtime, nil
}

func resolveLinuxBackend(configured string) string {
	choice := strings.ToLower(strings.TrimSpace(configured))
	if choice == "" {
		choice = "auto"
	}
	if choice == "evdev" {
		choice = "wayland"
	}
	if choice != "auto" {
		return choice
	}

	sessionType := strings.ToLower(strings.TrimSpace(os.Getenv("XDG_SESSION_TYPE")))
	switch sessionType {
	case "wayland":
		return "wayland"
	case "x11":
		return "x11"
	}

	if strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != "" {
		return "wayland"
	}
	if strings.TrimSpace(os.Getenv("DISPLAY")) != "" {
		return "x11"
	}
	return "wayland"
}
