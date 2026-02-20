package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

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

func TestWriteJSONArray(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSONArray(&buf, []string{"a", "b"}); err != nil {
		t.Fatalf("write json array: %v", err)
	}
	var got []string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode json array: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("unexpected array: %#v", got)
	}
}

func TestWriteNDJSONSlice(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteNDJSONSlice(&buf, []string{"a", "b"}); err != nil {
		t.Fatalf("write ndjson slice: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var first string
	if err := json.Unmarshal(lines[0], &first); err != nil {
		t.Fatalf("decode first line: %v", err)
	}
	if first != "a" {
		t.Fatalf("unexpected first value: %q", first)
	}
}

func TestDetectModePlainAndNDJSONConflict(t *testing.T) {
	if _, err := DetectMode(false, true, true, true); err == nil {
		t.Fatalf("expected conflict error")
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, map[string]any{"ok": true}, Meta{RequestID: "rid"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if got["ok"] != true {
		t.Fatalf("unexpected payload: %#v", got)
	}
}

func TestWritePlain(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePlain(&buf, [][]string{{"a", "b"}, {"c", "d"}}); err != nil {
		t.Fatalf("WritePlain: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "a\tb") || !strings.Contains(got, "c\td") {
		t.Fatalf("unexpected plain output: %q", got)
	}
}

func TestWriteNDJSON(t *testing.T) {
	var buf bytes.Buffer
	items := []any{map[string]any{"id": "1"}, map[string]any{"id": "2"}}
	if err := WriteNDJSON(&buf, items); err != nil {
		t.Fatalf("WriteNDJSON: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestWriteTableWithSanitizationAndPadding(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"ID", "Title"}
	rows := [][]string{
		{"1", "Hello\tWorld"},
		{"22", "Line1\nLine2"},
	}
	if err := WriteTable(&buf, headers, rows); err != nil {
		t.Fatalf("WriteTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Hello World") || !strings.Contains(out, "Line1 Line2") {
		t.Fatalf("expected sanitized cells, got %q", out)
	}
	if !strings.Contains(out, "ID  Title") {
		t.Fatalf("expected header row, got %q", out)
	}
}

func TestWriteTableNoColumns(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTable(&buf, nil, nil); err != nil {
		t.Fatalf("WriteTable: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected empty output, got %q", buf.String())
	}
}

func TestIsTTYForRegularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "output-test-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()
	if IsTTY(f) {
		t.Fatalf("expected temp file to not be tty")
	}
}

type failWriter struct{}

func (f failWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write fail")
}

func TestWritePlainPropagatesWriterError(t *testing.T) {
	err := WritePlain(failWriter{}, [][]string{{"a"}})
	if err == nil {
		t.Fatalf("expected writer error")
	}
}

func TestWriteNDJSONPropagatesWriterError(t *testing.T) {
	err := WriteNDJSON(failWriter{}, []any{"a"})
	if err == nil {
		t.Fatalf("expected writer error")
	}
}
