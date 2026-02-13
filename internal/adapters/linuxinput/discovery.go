//go:build linux

package linuxinput

import (
	"fmt"
	"os"
	"sort"
	"strings"

	evdev "github.com/holoplot/go-evdev"
)

type DeviceInfo struct {
	Path      string
	Name      string
	IsVirtual bool
	IsPointer bool
}

type SourceSelection struct {
	Devices      []*evdev.InputDevice
	TriggerPaths map[string]struct{}
	TogglePaths  map[string]struct{}
}

func ListInputDevices() ([]DeviceInfo, error) {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return nil, err
	}

	sort.Slice(paths, func(i, j int) bool {
		return paths[i].Path < paths[j].Path
	})

	devices := make([]DeviceInfo, 0, len(paths))
	for _, path := range paths {
		dev, err := openInputDevice(path.Path)
		if err != nil {
			continue
		}

		name := path.Name
		if actualName, err := dev.Name(); err == nil && actualName != "" {
			name = actualName
		}

		devices = append(devices, DeviceInfo{
			Path:      path.Path,
			Name:      name,
			IsVirtual: deviceIsVirtual(dev, name),
			IsPointer: deviceIsPointer(dev),
		})
		_ = dev.Close()
	}

	return devices, nil
}

func OpenSourceSelection(devicePath string, triggerCode, toggleCode uint16) (*SourceSelection, error) {
	if devicePath != "" {
		dev, err := openInputDevice(devicePath)
		if err != nil {
			return nil, err
		}
		if !deviceSupportsCode(dev, triggerCode) {
			_ = dev.Close()
			return nil, fmt.Errorf("%s does not expose trigger %s", devicePath, FormatCodeName(triggerCode))
		}
		if !deviceSupportsCode(dev, toggleCode) {
			_ = dev.Close()
			return nil, fmt.Errorf("%s does not expose toggle %s", devicePath, FormatCodeName(toggleCode))
		}
		path := dev.Path()
		return &SourceSelection{
			Devices:      []*evdev.InputDevice{dev},
			TriggerPaths: map[string]struct{}{path: {}},
			TogglePaths:  map[string]struct{}{path: {}},
		}, nil
	}

	triggerMatches, err := findDevicesByCode(triggerCode)
	if err != nil {
		return nil, err
	}
	if len(triggerMatches) == 0 {
		return nil, fmt.Errorf("no input device exposes trigger %s; use --list-devices and then pass --device", FormatCodeName(triggerCode))
	}

	toggleMatches, err := findDevicesByCode(toggleCode)
	if err != nil {
		return nil, err
	}
	if len(toggleMatches) == 0 {
		return nil, fmt.Errorf("no input device exposes toggle %s; use --list-devices and choose another --toggle", FormatCodeName(toggleCode))
	}

	triggerPaths := make(map[string]struct{}, len(triggerMatches))
	for _, dev := range triggerMatches {
		triggerPaths[dev.Path] = struct{}{}
	}

	togglePaths := make(map[string]struct{}, len(toggleMatches))
	for _, dev := range toggleMatches {
		togglePaths[dev.Path] = struct{}{}
	}

	allPathMap := make(map[string]struct{}, len(triggerPaths)+len(togglePaths))
	for path := range triggerPaths {
		allPathMap[path] = struct{}{}
	}
	for path := range togglePaths {
		allPathMap[path] = struct{}{}
	}

	allPaths := make([]string, 0, len(allPathMap))
	for path := range allPathMap {
		allPaths = append(allPaths, path)
	}
	sort.Strings(allPaths)

	devices := make([]*evdev.InputDevice, 0, len(allPaths))
	closeDevices := func() {
		for _, dev := range devices {
			_ = dev.Close()
		}
	}

	for _, path := range allPaths {
		dev, err := openInputDevice(path)
		if err != nil {
			continue
		}
		devices = append(devices, dev)
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("found matching input devices, but failed to open any of them")
	}

	opened := make(map[string]struct{}, len(devices))
	for _, dev := range devices {
		opened[dev.Path()] = struct{}{}
	}
	for path := range triggerPaths {
		if _, ok := opened[path]; !ok {
			delete(triggerPaths, path)
		}
	}
	for path := range togglePaths {
		if _, ok := opened[path]; !ok {
			delete(togglePaths, path)
		}
	}

	if len(triggerPaths) == 0 {
		closeDevices()
		return nil, fmt.Errorf("failed to open any trigger-capable input devices")
	}
	if len(togglePaths) == 0 {
		closeDevices()
		return nil, fmt.Errorf("failed to open any toggle-capable input devices")
	}

	return &SourceSelection{Devices: devices, TriggerPaths: triggerPaths, TogglePaths: togglePaths}, nil
}

func openInputDevice(path string) (*evdev.InputDevice, error) {
	return evdev.OpenWithFlags(path, os.O_RDONLY)
}

func deviceSupportsCode(device *evdev.InputDevice, code uint16) bool {
	needle := evdev.EvCode(code)
	for _, c := range device.CapableEvents(evdev.EV_KEY) {
		if c == needle {
			return true
		}
	}
	return false
}

func deviceIsVirtual(device *evdev.InputDevice, name string) bool {
	id, err := device.InputID()
	if err == nil && id.BusType == uint16(evdev.BUS_VIRTUAL) {
		return true
	}
	lower := strings.ToLower(name)
	for _, token := range []string{"virtual", "uinput", "ydotool", "hold-autoclicker", "autoclicker"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func deviceIsPointer(device *evdev.InputDevice) bool {
	var hasRelX, hasRelY bool
	for _, code := range device.CapableEvents(evdev.EV_REL) {
		if code == evdev.REL_X {
			hasRelX = true
		}
		if code == evdev.REL_Y {
			hasRelY = true
		}
	}
	if hasRelX && hasRelY {
		return true
	}
	return len(device.CapableEvents(evdev.EV_ABS)) > 0
}

func codeIsMouseButton(code uint16) bool {
	c := evdev.EvCode(code)
	return c >= evdev.BTN_MOUSE && c <= evdev.BTN_TASK
}

func deviceHasRelativeXY(device *evdev.InputDevice) bool {
	var hasRelX, hasRelY bool
	for _, code := range device.CapableEvents(evdev.EV_REL) {
		if code == evdev.REL_X {
			hasRelX = true
		}
		if code == evdev.REL_Y {
			hasRelY = true
		}
	}
	return hasRelX && hasRelY
}

func findDevicesByCode(code uint16) ([]DeviceInfo, error) {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return nil, err
	}

	matches := make([]DeviceInfo, 0)
	for _, path := range paths {
		dev, err := openInputDevice(path.Path)
		if err != nil {
			continue
		}

		name := path.Name
		if actualName, err := dev.Name(); err == nil && actualName != "" {
			name = actualName
		}
		if deviceSupportsCode(dev, code) {
			matches = append(matches, DeviceInfo{
				Path:      path.Path,
				Name:      name,
				IsVirtual: deviceIsVirtual(dev, name),
				IsPointer: deviceIsPointer(dev),
			})
		}
		_ = dev.Close()
	}

	if len(matches) == 0 {
		return matches, nil
	}

	pool := make([]DeviceInfo, 0, len(matches))
	for _, match := range matches {
		if !match.IsVirtual {
			pool = append(pool, match)
		}
	}
	if len(pool) == 0 {
		pool = matches
	}

	if codeIsMouseButton(code) {
		pointerPool := make([]DeviceInfo, 0, len(pool))
		for _, match := range pool {
			if match.IsPointer {
				pointerPool = append(pointerPool, match)
			}
		}
		if len(pointerPool) > 0 {
			pool = pointerPool
		}
	}

	sort.Slice(pool, func(i, j int) bool {
		return pool[i].Path < pool[j].Path
	})
	return pool, nil
}
