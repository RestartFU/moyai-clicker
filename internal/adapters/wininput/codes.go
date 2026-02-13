package wininput

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	CodeBTNLeft   uint16 = 0x110
	CodeBTNRight  uint16 = 0x111
	CodeBTNMiddle uint16 = 0x112
	CodeBTNSide   uint16 = 0x113
	CodeBTNExtra  uint16 = 0x114
)

const (
	codeKEYEsc        uint16 = 1
	codeKEY1          uint16 = 2
	codeKEY2          uint16 = 3
	codeKEY3          uint16 = 4
	codeKEY4          uint16 = 5
	codeKEY5          uint16 = 6
	codeKEY6          uint16 = 7
	codeKEY7          uint16 = 8
	codeKEY8          uint16 = 9
	codeKEY9          uint16 = 10
	codeKEY0          uint16 = 11
	codeKEYMinus      uint16 = 12
	codeKEYEqual      uint16 = 13
	codeKEYBackspace  uint16 = 14
	codeKEYTab        uint16 = 15
	codeKEYQ          uint16 = 16
	codeKEYW          uint16 = 17
	codeKEYE          uint16 = 18
	codeKEYR          uint16 = 19
	codeKEYT          uint16 = 20
	codeKEYY          uint16 = 21
	codeKEYU          uint16 = 22
	codeKEYI          uint16 = 23
	codeKEYO          uint16 = 24
	codeKEYP          uint16 = 25
	codeKEYLeftBrace  uint16 = 26
	codeKEYRightBrace uint16 = 27
	codeKEYEnter      uint16 = 28
	codeKEYLeftCtrl   uint16 = 29
	codeKEYA          uint16 = 30
	codeKEYS          uint16 = 31
	codeKEYD          uint16 = 32
	codeKEYF          uint16 = 33
	codeKEYG          uint16 = 34
	codeKEYH          uint16 = 35
	codeKEYJ          uint16 = 36
	codeKEYK          uint16 = 37
	codeKEYL          uint16 = 38
	codeKEYSemicolon  uint16 = 39
	codeKEYApostrophe uint16 = 40
	codeKEYGrave      uint16 = 41
	codeKEYLeftShift  uint16 = 42
	codeKEYBackslash  uint16 = 43
	codeKEYZ          uint16 = 44
	codeKEYX          uint16 = 45
	codeKEYC          uint16 = 46
	codeKEYV          uint16 = 47
	codeKEYB          uint16 = 48
	codeKEYN          uint16 = 49
	codeKEYM          uint16 = 50
	codeKEYComma      uint16 = 51
	codeKEYDot        uint16 = 52
	codeKEYSlash      uint16 = 53
	codeKEYRightShift uint16 = 54
	codeKEYKPAsterisk uint16 = 55
	codeKEYLeftAlt    uint16 = 56
	codeKEYSpace      uint16 = 57
	codeKEYCapsLock   uint16 = 58
	codeKEYF1         uint16 = 59
	codeKEYF2         uint16 = 60
	codeKEYF3         uint16 = 61
	codeKEYF4         uint16 = 62
	codeKEYF5         uint16 = 63
	codeKEYF6         uint16 = 64
	codeKEYF7         uint16 = 65
	codeKEYF8         uint16 = 66
	codeKEYF9         uint16 = 67
	codeKEYF10        uint16 = 68
	codeKEYNumLock    uint16 = 69
	codeKEYScrollLock uint16 = 70
	codeKEYKP7        uint16 = 71
	codeKEYKP8        uint16 = 72
	codeKEYKP9        uint16 = 73
	codeKEYKPMinus    uint16 = 74
	codeKEYKP4        uint16 = 75
	codeKEYKP5        uint16 = 76
	codeKEYKP6        uint16 = 77
	codeKEYKPPlus     uint16 = 78
	codeKEYKP1        uint16 = 79
	codeKEYKP2        uint16 = 80
	codeKEYKP3        uint16 = 81
	codeKEYKP0        uint16 = 82
	codeKEYKPDot      uint16 = 83
	codeKEYF11        uint16 = 87
	codeKEYF12        uint16 = 88
	codeKEYKPEnter    uint16 = 96
	codeKEYRightCtrl  uint16 = 97
	codeKEYKPSlash    uint16 = 98
	codeKEYSysRq      uint16 = 99
	codeKEYRightAlt   uint16 = 100
	codeKEYHome       uint16 = 102
	codeKEYUp         uint16 = 103
	codeKEYPageUp     uint16 = 104
	codeKEYLeft       uint16 = 105
	codeKEYRight      uint16 = 106
	codeKEYEnd        uint16 = 107
	codeKEYDown       uint16 = 108
	codeKEYPageDown   uint16 = 109
	codeKEYInsert     uint16 = 110
	codeKEYDelete     uint16 = 111
	codeKEYMute       uint16 = 113
	codeKEYVolumeDown uint16 = 114
	codeKEYVolumeUp   uint16 = 115
	codeKEYPause      uint16 = 119
	codeKEYLeftMeta   uint16 = 125
	codeKEYRightMeta  uint16 = 126
	codeKEYMenu       uint16 = 139
	codeKEYF13        uint16 = 183
	codeKEYF14        uint16 = 184
	codeKEYF15        uint16 = 185
	codeKEYF16        uint16 = 186
	codeKEYF17        uint16 = 187
	codeKEYF18        uint16 = 188
	codeKEYF19        uint16 = 189
	codeKEYF20        uint16 = 190
	codeKEYF21        uint16 = 191
	codeKEYF22        uint16 = 192
	codeKEYF23        uint16 = 193
	codeKEYF24        uint16 = 194
)

const (
	vkLBUTTON  uint32 = 0x01
	vkRBUTTON  uint32 = 0x02
	vkMBUTTON  uint32 = 0x04
	vkXBUTTON1 uint32 = 0x05
	vkXBUTTON2 uint32 = 0x06

	vkBACK       uint32 = 0x08
	vkTAB        uint32 = 0x09
	vkRETURN     uint32 = 0x0D
	vkSHIFT      uint32 = 0x10
	vkCONTROL    uint32 = 0x11
	vkMENU       uint32 = 0x12
	vkPAUSE      uint32 = 0x13
	vkCAPITAL    uint32 = 0x14
	vkESCAPE     uint32 = 0x1B
	vkSPACE      uint32 = 0x20
	vkPRIOR      uint32 = 0x21
	vkNEXT       uint32 = 0x22
	vkEND        uint32 = 0x23
	vkHOME       uint32 = 0x24
	vkLEFT       uint32 = 0x25
	vkUP         uint32 = 0x26
	vkRIGHT      uint32 = 0x27
	vkDOWN       uint32 = 0x28
	vkSNAPSHOT   uint32 = 0x2C
	vkINSERT     uint32 = 0x2D
	vkDELETE     uint32 = 0x2E
	vk0          uint32 = 0x30
	vk1          uint32 = 0x31
	vk2          uint32 = 0x32
	vk3          uint32 = 0x33
	vk4          uint32 = 0x34
	vk5          uint32 = 0x35
	vk6          uint32 = 0x36
	vk7          uint32 = 0x37
	vk8          uint32 = 0x38
	vk9          uint32 = 0x39
	vkA          uint32 = 0x41
	vkB          uint32 = 0x42
	vkC          uint32 = 0x43
	vkD          uint32 = 0x44
	vkE          uint32 = 0x45
	vkF          uint32 = 0x46
	vkG          uint32 = 0x47
	vkH          uint32 = 0x48
	vkI          uint32 = 0x49
	vkJ          uint32 = 0x4A
	vkK          uint32 = 0x4B
	vkL          uint32 = 0x4C
	vkM          uint32 = 0x4D
	vkN          uint32 = 0x4E
	vkO          uint32 = 0x4F
	vkP          uint32 = 0x50
	vkQ          uint32 = 0x51
	vkR          uint32 = 0x52
	vkS          uint32 = 0x53
	vkT          uint32 = 0x54
	vkU          uint32 = 0x55
	vkV          uint32 = 0x56
	vkW          uint32 = 0x57
	vkX          uint32 = 0x58
	vkY          uint32 = 0x59
	vkZ          uint32 = 0x5A
	vkLWIN       uint32 = 0x5B
	vkRWIN       uint32 = 0x5C
	vkAPPS       uint32 = 0x5D
	vkNUMPAD0    uint32 = 0x60
	vkNUMPAD1    uint32 = 0x61
	vkNUMPAD2    uint32 = 0x62
	vkNUMPAD3    uint32 = 0x63
	vkNUMPAD4    uint32 = 0x64
	vkNUMPAD5    uint32 = 0x65
	vkNUMPAD6    uint32 = 0x66
	vkNUMPAD7    uint32 = 0x67
	vkNUMPAD8    uint32 = 0x68
	vkNUMPAD9    uint32 = 0x69
	vkMULTIPLY   uint32 = 0x6A
	vkADD        uint32 = 0x6B
	vkSUBTRACT   uint32 = 0x6D
	vkDECIMAL    uint32 = 0x6E
	vkDIVIDE     uint32 = 0x6F
	vkF1         uint32 = 0x70
	vkF2         uint32 = 0x71
	vkF3         uint32 = 0x72
	vkF4         uint32 = 0x73
	vkF5         uint32 = 0x74
	vkF6         uint32 = 0x75
	vkF7         uint32 = 0x76
	vkF8         uint32 = 0x77
	vkF9         uint32 = 0x78
	vkF10        uint32 = 0x79
	vkF11        uint32 = 0x7A
	vkF12        uint32 = 0x7B
	vkF13        uint32 = 0x7C
	vkF14        uint32 = 0x7D
	vkF15        uint32 = 0x7E
	vkF16        uint32 = 0x7F
	vkF17        uint32 = 0x80
	vkF18        uint32 = 0x81
	vkF19        uint32 = 0x82
	vkF20        uint32 = 0x83
	vkF21        uint32 = 0x84
	vkF22        uint32 = 0x85
	vkF23        uint32 = 0x86
	vkF24        uint32 = 0x87
	vkNUMLOCK    uint32 = 0x90
	vkSCROLL     uint32 = 0x91
	vkLSHIFT     uint32 = 0xA0
	vkRSHIFT     uint32 = 0xA1
	vkLCONTROL   uint32 = 0xA2
	vkRCONTROL   uint32 = 0xA3
	vkLMENU      uint32 = 0xA4
	vkRMENU      uint32 = 0xA5
	vkVOLUMEMUTE uint32 = 0xAD
	vkVOLUMEDOWN uint32 = 0xAE
	vkVOLUMEUP   uint32 = 0xAF
	vkOEM1       uint32 = 0xBA
	vkOEMPLUS    uint32 = 0xBB
	vkOEMCOMMA   uint32 = 0xBC
	vkOEMMINUS   uint32 = 0xBD
	vkOEMPERIOD  uint32 = 0xBE
	vkOEM2       uint32 = 0xBF
	vkOEM3       uint32 = 0xC0
	vkOEM4       uint32 = 0xDB
	vkOEM5       uint32 = 0xDC
	vkOEM6       uint32 = 0xDD
	vkOEM7       uint32 = 0xDE
)

const (
	llkhfExtended = 0x01
)

var codeNameToCode = map[string]uint16{
	"BTN_LEFT":    CodeBTNLeft,
	"BTN_RIGHT":   CodeBTNRight,
	"BTN_MIDDLE":  CodeBTNMiddle,
	"BTN_SIDE":    CodeBTNSide,
	"BTN_BACK":    CodeBTNSide,
	"BTN_EXTRA":   CodeBTNExtra,
	"BTN_FORWARD": CodeBTNExtra,

	"KEY_ESC":        codeKEYEsc,
	"KEY_1":          codeKEY1,
	"KEY_2":          codeKEY2,
	"KEY_3":          codeKEY3,
	"KEY_4":          codeKEY4,
	"KEY_5":          codeKEY5,
	"KEY_6":          codeKEY6,
	"KEY_7":          codeKEY7,
	"KEY_8":          codeKEY8,
	"KEY_9":          codeKEY9,
	"KEY_0":          codeKEY0,
	"KEY_MINUS":      codeKEYMinus,
	"KEY_EQUAL":      codeKEYEqual,
	"KEY_BACKSPACE":  codeKEYBackspace,
	"KEY_TAB":        codeKEYTab,
	"KEY_Q":          codeKEYQ,
	"KEY_W":          codeKEYW,
	"KEY_E":          codeKEYE,
	"KEY_R":          codeKEYR,
	"KEY_T":          codeKEYT,
	"KEY_Y":          codeKEYY,
	"KEY_U":          codeKEYU,
	"KEY_I":          codeKEYI,
	"KEY_O":          codeKEYO,
	"KEY_P":          codeKEYP,
	"KEY_LEFTBRACE":  codeKEYLeftBrace,
	"KEY_RIGHTBRACE": codeKEYRightBrace,
	"KEY_ENTER":      codeKEYEnter,
	"KEY_LEFTCTRL":   codeKEYLeftCtrl,
	"KEY_A":          codeKEYA,
	"KEY_S":          codeKEYS,
	"KEY_D":          codeKEYD,
	"KEY_F":          codeKEYF,
	"KEY_G":          codeKEYG,
	"KEY_H":          codeKEYH,
	"KEY_J":          codeKEYJ,
	"KEY_K":          codeKEYK,
	"KEY_L":          codeKEYL,
	"KEY_SEMICOLON":  codeKEYSemicolon,
	"KEY_APOSTROPHE": codeKEYApostrophe,
	"KEY_GRAVE":      codeKEYGrave,
	"KEY_LEFTSHIFT":  codeKEYLeftShift,
	"KEY_BACKSLASH":  codeKEYBackslash,
	"KEY_Z":          codeKEYZ,
	"KEY_X":          codeKEYX,
	"KEY_C":          codeKEYC,
	"KEY_V":          codeKEYV,
	"KEY_B":          codeKEYB,
	"KEY_N":          codeKEYN,
	"KEY_M":          codeKEYM,
	"KEY_COMMA":      codeKEYComma,
	"KEY_DOT":        codeKEYDot,
	"KEY_SLASH":      codeKEYSlash,
	"KEY_RIGHTSHIFT": codeKEYRightShift,
	"KEY_KPASTERISK": codeKEYKPAsterisk,
	"KEY_LEFTALT":    codeKEYLeftAlt,
	"KEY_SPACE":      codeKEYSpace,
	"KEY_CAPSLOCK":   codeKEYCapsLock,
	"KEY_F1":         codeKEYF1,
	"KEY_F2":         codeKEYF2,
	"KEY_F3":         codeKEYF3,
	"KEY_F4":         codeKEYF4,
	"KEY_F5":         codeKEYF5,
	"KEY_F6":         codeKEYF6,
	"KEY_F7":         codeKEYF7,
	"KEY_F8":         codeKEYF8,
	"KEY_F9":         codeKEYF9,
	"KEY_F10":        codeKEYF10,
	"KEY_NUMLOCK":    codeKEYNumLock,
	"KEY_SCROLLLOCK": codeKEYScrollLock,
	"KEY_KP7":        codeKEYKP7,
	"KEY_KP8":        codeKEYKP8,
	"KEY_KP9":        codeKEYKP9,
	"KEY_KPMINUS":    codeKEYKPMinus,
	"KEY_KP4":        codeKEYKP4,
	"KEY_KP5":        codeKEYKP5,
	"KEY_KP6":        codeKEYKP6,
	"KEY_KPPLUS":     codeKEYKPPlus,
	"KEY_KP1":        codeKEYKP1,
	"KEY_KP2":        codeKEYKP2,
	"KEY_KP3":        codeKEYKP3,
	"KEY_KP0":        codeKEYKP0,
	"KEY_KPDOT":      codeKEYKPDot,
	"KEY_F11":        codeKEYF11,
	"KEY_F12":        codeKEYF12,
	"KEY_KPENTER":    codeKEYKPEnter,
	"KEY_RIGHTCTRL":  codeKEYRightCtrl,
	"KEY_KPSLASH":    codeKEYKPSlash,
	"KEY_SYSRQ":      codeKEYSysRq,
	"KEY_RIGHTALT":   codeKEYRightAlt,
	"KEY_HOME":       codeKEYHome,
	"KEY_UP":         codeKEYUp,
	"KEY_PAGEUP":     codeKEYPageUp,
	"KEY_LEFT":       codeKEYLeft,
	"KEY_RIGHT":      codeKEYRight,
	"KEY_END":        codeKEYEnd,
	"KEY_DOWN":       codeKEYDown,
	"KEY_PAGEDOWN":   codeKEYPageDown,
	"KEY_INSERT":     codeKEYInsert,
	"KEY_DELETE":     codeKEYDelete,
	"KEY_MUTE":       codeKEYMute,
	"KEY_VOLUMEDOWN": codeKEYVolumeDown,
	"KEY_VOLUMEUP":   codeKEYVolumeUp,
	"KEY_PAUSE":      codeKEYPause,
	"KEY_LEFTMETA":   codeKEYLeftMeta,
	"KEY_RIGHTMETA":  codeKEYRightMeta,
	"KEY_MENU":       codeKEYMenu,
	"KEY_F13":        codeKEYF13,
	"KEY_F14":        codeKEYF14,
	"KEY_F15":        codeKEYF15,
	"KEY_F16":        codeKEYF16,
	"KEY_F17":        codeKEYF17,
	"KEY_F18":        codeKEYF18,
	"KEY_F19":        codeKEYF19,
	"KEY_F20":        codeKEYF20,
	"KEY_F21":        codeKEYF21,
	"KEY_F22":        codeKEYF22,
	"KEY_F23":        codeKEYF23,
	"KEY_F24":        codeKEYF24,
}

var codeToName = map[uint16]string{
	CodeBTNLeft:   "BTN_LEFT",
	CodeBTNRight:  "BTN_RIGHT",
	CodeBTNMiddle: "BTN_MIDDLE",
	CodeBTNSide:   "BTN_SIDE",
	CodeBTNExtra:  "BTN_EXTRA",

	codeKEYEsc:        "KEY_ESC",
	codeKEY1:          "KEY_1",
	codeKEY2:          "KEY_2",
	codeKEY3:          "KEY_3",
	codeKEY4:          "KEY_4",
	codeKEY5:          "KEY_5",
	codeKEY6:          "KEY_6",
	codeKEY7:          "KEY_7",
	codeKEY8:          "KEY_8",
	codeKEY9:          "KEY_9",
	codeKEY0:          "KEY_0",
	codeKEYMinus:      "KEY_MINUS",
	codeKEYEqual:      "KEY_EQUAL",
	codeKEYBackspace:  "KEY_BACKSPACE",
	codeKEYTab:        "KEY_TAB",
	codeKEYQ:          "KEY_Q",
	codeKEYW:          "KEY_W",
	codeKEYE:          "KEY_E",
	codeKEYR:          "KEY_R",
	codeKEYT:          "KEY_T",
	codeKEYY:          "KEY_Y",
	codeKEYU:          "KEY_U",
	codeKEYI:          "KEY_I",
	codeKEYO:          "KEY_O",
	codeKEYP:          "KEY_P",
	codeKEYLeftBrace:  "KEY_LEFTBRACE",
	codeKEYRightBrace: "KEY_RIGHTBRACE",
	codeKEYEnter:      "KEY_ENTER",
	codeKEYLeftCtrl:   "KEY_LEFTCTRL",
	codeKEYA:          "KEY_A",
	codeKEYS:          "KEY_S",
	codeKEYD:          "KEY_D",
	codeKEYF:          "KEY_F",
	codeKEYG:          "KEY_G",
	codeKEYH:          "KEY_H",
	codeKEYJ:          "KEY_J",
	codeKEYK:          "KEY_K",
	codeKEYL:          "KEY_L",
	codeKEYSemicolon:  "KEY_SEMICOLON",
	codeKEYApostrophe: "KEY_APOSTROPHE",
	codeKEYGrave:      "KEY_GRAVE",
	codeKEYLeftShift:  "KEY_LEFTSHIFT",
	codeKEYBackslash:  "KEY_BACKSLASH",
	codeKEYZ:          "KEY_Z",
	codeKEYX:          "KEY_X",
	codeKEYC:          "KEY_C",
	codeKEYV:          "KEY_V",
	codeKEYB:          "KEY_B",
	codeKEYN:          "KEY_N",
	codeKEYM:          "KEY_M",
	codeKEYComma:      "KEY_COMMA",
	codeKEYDot:        "KEY_DOT",
	codeKEYSlash:      "KEY_SLASH",
	codeKEYRightShift: "KEY_RIGHTSHIFT",
	codeKEYKPAsterisk: "KEY_KPASTERISK",
	codeKEYLeftAlt:    "KEY_LEFTALT",
	codeKEYSpace:      "KEY_SPACE",
	codeKEYCapsLock:   "KEY_CAPSLOCK",
	codeKEYF1:         "KEY_F1",
	codeKEYF2:         "KEY_F2",
	codeKEYF3:         "KEY_F3",
	codeKEYF4:         "KEY_F4",
	codeKEYF5:         "KEY_F5",
	codeKEYF6:         "KEY_F6",
	codeKEYF7:         "KEY_F7",
	codeKEYF8:         "KEY_F8",
	codeKEYF9:         "KEY_F9",
	codeKEYF10:        "KEY_F10",
	codeKEYNumLock:    "KEY_NUMLOCK",
	codeKEYScrollLock: "KEY_SCROLLLOCK",
	codeKEYKP7:        "KEY_KP7",
	codeKEYKP8:        "KEY_KP8",
	codeKEYKP9:        "KEY_KP9",
	codeKEYKPMinus:    "KEY_KPMINUS",
	codeKEYKP4:        "KEY_KP4",
	codeKEYKP5:        "KEY_KP5",
	codeKEYKP6:        "KEY_KP6",
	codeKEYKPPlus:     "KEY_KPPLUS",
	codeKEYKP1:        "KEY_KP1",
	codeKEYKP2:        "KEY_KP2",
	codeKEYKP3:        "KEY_KP3",
	codeKEYKP0:        "KEY_KP0",
	codeKEYKPDot:      "KEY_KPDOT",
	codeKEYF11:        "KEY_F11",
	codeKEYF12:        "KEY_F12",
	codeKEYKPEnter:    "KEY_KPENTER",
	codeKEYRightCtrl:  "KEY_RIGHTCTRL",
	codeKEYKPSlash:    "KEY_KPSLASH",
	codeKEYSysRq:      "KEY_SYSRQ",
	codeKEYRightAlt:   "KEY_RIGHTALT",
	codeKEYHome:       "KEY_HOME",
	codeKEYUp:         "KEY_UP",
	codeKEYPageUp:     "KEY_PAGEUP",
	codeKEYLeft:       "KEY_LEFT",
	codeKEYRight:      "KEY_RIGHT",
	codeKEYEnd:        "KEY_END",
	codeKEYDown:       "KEY_DOWN",
	codeKEYPageDown:   "KEY_PAGEDOWN",
	codeKEYInsert:     "KEY_INSERT",
	codeKEYDelete:     "KEY_DELETE",
	codeKEYMute:       "KEY_MUTE",
	codeKEYVolumeDown: "KEY_VOLUMEDOWN",
	codeKEYVolumeUp:   "KEY_VOLUMEUP",
	codeKEYPause:      "KEY_PAUSE",
	codeKEYLeftMeta:   "KEY_LEFTMETA",
	codeKEYRightMeta:  "KEY_RIGHTMETA",
	codeKEYMenu:       "KEY_MENU",
	codeKEYF13:        "KEY_F13",
	codeKEYF14:        "KEY_F14",
	codeKEYF15:        "KEY_F15",
	codeKEYF16:        "KEY_F16",
	codeKEYF17:        "KEY_F17",
	codeKEYF18:        "KEY_F18",
	codeKEYF19:        "KEY_F19",
	codeKEYF20:        "KEY_F20",
	codeKEYF21:        "KEY_F21",
	codeKEYF22:        "KEY_F22",
	codeKEYF23:        "KEY_F23",
	codeKEYF24:        "KEY_F24",
}

var codeToVK = map[uint16]uint32{
	CodeBTNLeft:   vkLBUTTON,
	CodeBTNRight:  vkRBUTTON,
	CodeBTNMiddle: vkMBUTTON,
	CodeBTNSide:   vkXBUTTON1,
	CodeBTNExtra:  vkXBUTTON2,

	codeKEYEsc:        vkESCAPE,
	codeKEY1:          vk1,
	codeKEY2:          vk2,
	codeKEY3:          vk3,
	codeKEY4:          vk4,
	codeKEY5:          vk5,
	codeKEY6:          vk6,
	codeKEY7:          vk7,
	codeKEY8:          vk8,
	codeKEY9:          vk9,
	codeKEY0:          vk0,
	codeKEYMinus:      vkOEMMINUS,
	codeKEYEqual:      vkOEMPLUS,
	codeKEYBackspace:  vkBACK,
	codeKEYTab:        vkTAB,
	codeKEYQ:          vkQ,
	codeKEYW:          vkW,
	codeKEYE:          vkE,
	codeKEYR:          vkR,
	codeKEYT:          vkT,
	codeKEYY:          vkY,
	codeKEYU:          vkU,
	codeKEYI:          vkI,
	codeKEYO:          vkO,
	codeKEYP:          vkP,
	codeKEYLeftBrace:  vkOEM4,
	codeKEYRightBrace: vkOEM6,
	codeKEYEnter:      vkRETURN,
	codeKEYLeftCtrl:   vkLCONTROL,
	codeKEYA:          vkA,
	codeKEYS:          vkS,
	codeKEYD:          vkD,
	codeKEYF:          vkF,
	codeKEYG:          vkG,
	codeKEYH:          vkH,
	codeKEYJ:          vkJ,
	codeKEYK:          vkK,
	codeKEYL:          vkL,
	codeKEYSemicolon:  vkOEM1,
	codeKEYApostrophe: vkOEM7,
	codeKEYGrave:      vkOEM3,
	codeKEYLeftShift:  vkLSHIFT,
	codeKEYBackslash:  vkOEM5,
	codeKEYZ:          vkZ,
	codeKEYX:          vkX,
	codeKEYC:          vkC,
	codeKEYV:          vkV,
	codeKEYB:          vkB,
	codeKEYN:          vkN,
	codeKEYM:          vkM,
	codeKEYComma:      vkOEMCOMMA,
	codeKEYDot:        vkOEMPERIOD,
	codeKEYSlash:      vkOEM2,
	codeKEYRightShift: vkRSHIFT,
	codeKEYKPAsterisk: vkMULTIPLY,
	codeKEYLeftAlt:    vkLMENU,
	codeKEYSpace:      vkSPACE,
	codeKEYCapsLock:   vkCAPITAL,
	codeKEYF1:         vkF1,
	codeKEYF2:         vkF2,
	codeKEYF3:         vkF3,
	codeKEYF4:         vkF4,
	codeKEYF5:         vkF5,
	codeKEYF6:         vkF6,
	codeKEYF7:         vkF7,
	codeKEYF8:         vkF8,
	codeKEYF9:         vkF9,
	codeKEYF10:        vkF10,
	codeKEYNumLock:    vkNUMLOCK,
	codeKEYScrollLock: vkSCROLL,
	codeKEYKP7:        vkNUMPAD7,
	codeKEYKP8:        vkNUMPAD8,
	codeKEYKP9:        vkNUMPAD9,
	codeKEYKPMinus:    vkSUBTRACT,
	codeKEYKP4:        vkNUMPAD4,
	codeKEYKP5:        vkNUMPAD5,
	codeKEYKP6:        vkNUMPAD6,
	codeKEYKPPlus:     vkADD,
	codeKEYKP1:        vkNUMPAD1,
	codeKEYKP2:        vkNUMPAD2,
	codeKEYKP3:        vkNUMPAD3,
	codeKEYKP0:        vkNUMPAD0,
	codeKEYKPDot:      vkDECIMAL,
	codeKEYF11:        vkF11,
	codeKEYF12:        vkF12,
	codeKEYKPEnter:    vkRETURN,
	codeKEYRightCtrl:  vkRCONTROL,
	codeKEYKPSlash:    vkDIVIDE,
	codeKEYSysRq:      vkSNAPSHOT,
	codeKEYRightAlt:   vkRMENU,
	codeKEYHome:       vkHOME,
	codeKEYUp:         vkUP,
	codeKEYPageUp:     vkPRIOR,
	codeKEYLeft:       vkLEFT,
	codeKEYRight:      vkRIGHT,
	codeKEYEnd:        vkEND,
	codeKEYDown:       vkDOWN,
	codeKEYPageDown:   vkNEXT,
	codeKEYInsert:     vkINSERT,
	codeKEYDelete:     vkDELETE,
	codeKEYMute:       vkVOLUMEMUTE,
	codeKEYVolumeDown: vkVOLUMEDOWN,
	codeKEYVolumeUp:   vkVOLUMEUP,
	codeKEYPause:      vkPAUSE,
	codeKEYLeftMeta:   vkLWIN,
	codeKEYRightMeta:  vkRWIN,
	codeKEYMenu:       vkAPPS,
	codeKEYF13:        vkF13,
	codeKEYF14:        vkF14,
	codeKEYF15:        vkF15,
	codeKEYF16:        vkF16,
	codeKEYF17:        vkF17,
	codeKEYF18:        vkF18,
	codeKEYF19:        vkF19,
	codeKEYF20:        vkF20,
	codeKEYF21:        vkF21,
	codeKEYF22:        vkF22,
	codeKEYF23:        vkF23,
	codeKEYF24:        vkF24,
}

var captureCodes []uint16
var vkToCode map[uint32]uint16

func init() {
	captureCodes = make([]uint16, 0, len(codeToVK))
	for code := range codeToVK {
		captureCodes = append(captureCodes, code)
	}
	sort.Slice(captureCodes, func(i, j int) bool { return captureCodes[i] < captureCodes[j] })

	vkToCode = make(map[uint32]uint16, len(codeToVK))
	for _, code := range captureCodes {
		vk := codeToVK[code]
		if _, exists := vkToCode[vk]; exists {
			continue
		}
		vkToCode[vk] = code
	}
}

func ParseCode(value string) (uint16, error) {
	raw := strings.ToUpper(strings.TrimSpace(value))
	if raw == "" {
		return 0, fmt.Errorf("trigger code is empty")
	}
	if code, ok := codeNameToCode[raw]; ok {
		return code, nil
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
	if name, ok := codeToName[code]; ok {
		return name
	}
	return strconv.Itoa(int(code))
}

func CodeToVK(code uint16) (uint32, bool) {
	vk, ok := codeToVK[code]
	return vk, ok
}

func CodeFromVK(vk, flags, _ uint32) (uint16, bool) {
	switch vk {
	case vkRETURN:
		if flags&llkhfExtended != 0 {
			return codeKEYKPEnter, true
		}
		return codeKEYEnter, true
	case vkSHIFT:
		return codeKEYLeftShift, true
	case vkCONTROL:
		if flags&llkhfExtended != 0 {
			return codeKEYRightCtrl, true
		}
		return codeKEYLeftCtrl, true
	case vkMENU:
		if flags&llkhfExtended != 0 {
			return codeKEYRightAlt, true
		}
		return codeKEYLeftAlt, true
	}

	code, ok := vkToCode[vk]
	if ok {
		return code, true
	}
	return 0, false
}

func CaptureCandidateCodes() []uint16 {
	out := make([]uint16, len(captureCodes))
	copy(out, captureCodes)
	return out
}
