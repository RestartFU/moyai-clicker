package wininput

import "testing"

func TestParseAndFormatMouseCodes(t *testing.T) {
	tests := []struct {
		raw      string
		expected uint16
	}{
		{raw: "BTN_LEFT", expected: CodeBTNLeft},
		{raw: "btn_extra", expected: CodeBTNExtra},
		{raw: "BTN_BACK", expected: CodeBTNSide},
		{raw: "BTN_FORWARD", expected: CodeBTNExtra},
	}

	for _, tc := range tests {
		got, err := ParseCode(tc.raw)
		if err != nil {
			t.Fatalf("ParseCode(%q) returned error: %v", tc.raw, err)
		}
		if got != tc.expected {
			t.Fatalf("ParseCode(%q)=%d, want %d", tc.raw, got, tc.expected)
		}
	}

	if name := FormatCodeName(CodeBTNExtra); name != "BTN_EXTRA" {
		t.Fatalf("FormatCodeName(CodeBTNExtra)=%q, want BTN_EXTRA", name)
	}
}

func TestCodeFromVKMappings(t *testing.T) {
	if code, ok := CodeFromVK(vkA, 0, 0); !ok || code != codeKEYA {
		t.Fatalf("CodeFromVK(vkA)=%d,%v, want %d,true", code, ok, codeKEYA)
	}

	if code, ok := CodeFromVK(vkRETURN, 0, 0); !ok || code != codeKEYEnter {
		t.Fatalf("CodeFromVK(vkRETURN)=%d,%v, want %d,true", code, ok, codeKEYEnter)
	}

	if code, ok := CodeFromVK(vkRETURN, llkhfExtended, 0); !ok || code != codeKEYKPEnter {
		t.Fatalf("CodeFromVK(vkRETURN,extended)=%d,%v, want %d,true", code, ok, codeKEYKPEnter)
	}
}

func TestCodeToVKMappings(t *testing.T) {
	if vk, ok := CodeToVK(codeKEYF8); !ok || vk != vkF8 {
		t.Fatalf("CodeToVK(KEY_F8)=%d,%v, want %d,true", vk, ok, vkF8)
	}
	if vk, ok := CodeToVK(CodeBTNSide); !ok || vk != vkXBUTTON1 {
		t.Fatalf("CodeToVK(BTN_SIDE)=%d,%v, want %d,true", vk, ok, vkXBUTTON1)
	}
}
