//go:build linux

package linuxinput

import (
	"fmt"
	"strconv"
	"strings"

	evdev "github.com/holoplot/go-evdev"
)

const (
	CodeBTNLeft  uint16 = uint16(evdev.BTN_LEFT)
	CodeBTNExtra uint16 = uint16(evdev.BTN_EXTRA)
)

func ParseCode(value string) (uint16, error) {
	raw := strings.ToUpper(strings.TrimSpace(value))
	if raw == "" {
		return 0, fmt.Errorf("trigger code is empty")
	}
	if code, ok := evdev.KEYFromString[raw]; ok {
		return uint16(code), nil
	}
	if strings.Contains(raw, "/") {
		parts := strings.Split(raw, "/")
		for i := len(parts) - 1; i >= 0; i-- {
			part := strings.TrimSpace(parts[i])
			if part == "" {
				continue
			}
			if code, ok := evdev.KEYFromString[part]; ok {
				return uint16(code), nil
			}
		}
	}

	parsed, err := strconv.ParseInt(raw, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("unknown trigger %q: use names like KEY_F8/BTN_SIDE or numeric code", value)
	}
	if parsed < 0 || parsed > 0xFFFF {
		return 0, fmt.Errorf("trigger code out of range: %d", parsed)
	}
	return uint16(parsed), nil
}

func FormatCodeName(code uint16) string {
	name := evdev.CodeName(evdev.EV_KEY, evdev.EvCode(code))
	if name != "" {
		if strings.Contains(name, "/") {
			parts := strings.Split(name, "/")
			for i := len(parts) - 1; i >= 0; i-- {
				part := strings.TrimSpace(parts[i])
				if strings.HasPrefix(part, "KEY_") || strings.HasPrefix(part, "BTN_") {
					return part
				}
			}
			return strings.TrimSpace(parts[len(parts)-1])
		}
		return name
	}
	return strconv.Itoa(int(code))
}
