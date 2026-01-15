package output

import "testing"

func TestDetectMode(t *testing.T) {
	cases := []struct {
		name       string
		jsonFlag   bool
		plainFlag  bool
		ndjsonFlag bool
		stdoutTTY  bool
		wantMode   Mode
		wantErr    bool
	}{
		{"json wins", true, false, false, true, ModeJSON, false},
		{"plain flag", false, true, false, true, ModePlain, false},
		{"ndjson flag", false, false, true, true, ModeNDJSON, false},
		{"json and plain conflict", true, true, false, true, "", true},
		{"json and ndjson conflict", true, false, true, true, "", true},
		{"non-tty defaults to plain", false, false, false, false, ModePlain, false},
		{"tty defaults to human", false, false, false, true, ModeHuman, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mode, err := DetectMode(tc.jsonFlag, tc.plainFlag, tc.ndjsonFlag, tc.stdoutTTY)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mode != tc.wantMode {
				t.Fatalf("mode=%s, want %s", mode, tc.wantMode)
			}
		})
	}
}
