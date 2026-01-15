package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

type Mode string

const (
	ModeHuman  Mode = "human"
	ModePlain  Mode = "plain"
	ModeJSON   Mode = "json"
	ModeNDJSON Mode = "ndjson"
)

type Meta struct {
	RequestID string `json:"request_id,omitempty"`
	Count     int    `json:"count,omitempty"`
	Cursor    string `json:"next_cursor,omitempty"`
}

type Envelope struct {
	Data any  `json:"data"`
	Meta Meta `json:"meta"`
}

func DetectMode(jsonFlag, plainFlag, ndjsonFlag bool, stdoutIsTTY bool) (Mode, error) {
	if (jsonFlag && plainFlag) || (jsonFlag && ndjsonFlag) || (plainFlag && ndjsonFlag) {
		return "", fmt.Errorf("--json, --plain, and --ndjson are mutually exclusive")
	}
	if ndjsonFlag {
		return ModeNDJSON, nil
	}
	if jsonFlag {
		return ModeJSON, nil
	}
	if plainFlag || !stdoutIsTTY {
		return ModePlain, nil
	}
	return ModeHuman, nil
}

func IsTTY(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func WriteJSON(out io.Writer, data any, meta Meta) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(Envelope{Data: data, Meta: meta})
}

func WritePlain(out io.Writer, rows [][]string) error {
	for _, row := range rows {
		if _, err := fmt.Fprintln(out, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return nil
}

func WriteNDJSON(out io.Writer, items []any) error {
	enc := json.NewEncoder(out)
	for _, item := range items {
		if err := enc.Encode(item); err != nil {
			return err
		}
	}
	return nil
}

func WriteTable(out io.Writer, headers []string, rows [][]string) error {
	cols := tableColumns(headers, rows)
	if cols == 0 {
		return nil
	}
	widths := make([]int, cols)
	updateWidths(widths, headers)
	for _, row := range rows {
		updateWidths(widths, row)
	}
	if len(headers) > 0 {
		if err := writeTableRow(out, headers, widths); err != nil {
			return err
		}
		if err := writeTableSeparator(out, widths); err != nil {
			return err
		}
	}
	for _, row := range rows {
		if err := writeTableRow(out, row, widths); err != nil {
			return err
		}
	}
	return nil
}

func tableColumns(headers []string, rows [][]string) int {
	cols := len(headers)
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	return cols
}

func updateWidths(widths []int, row []string) {
	for i := 0; i < len(widths); i++ {
		cell := ""
		if i < len(row) {
			cell = sanitizeCell(row[i])
		}
		if size := utf8.RuneCountInString(cell); size > widths[i] {
			widths[i] = size
		}
	}
}

func writeTableRow(out io.Writer, row []string, widths []int) error {
	var b strings.Builder
	for i := 0; i < len(widths); i++ {
		if i > 0 {
			b.WriteString("  ")
		}
		cell := ""
		if i < len(row) {
			cell = sanitizeCell(row[i])
		}
		b.WriteString(cell)
		if padding := widths[i] - utf8.RuneCountInString(cell); padding > 0 {
			b.WriteString(strings.Repeat(" ", padding))
		}
	}
	_, err := fmt.Fprintln(out, b.String())
	return err
}

func writeTableSeparator(out io.Writer, widths []int) error {
	var b strings.Builder
	for i, width := range widths {
		if i > 0 {
			b.WriteString("  ")
		}
		if width > 0 {
			b.WriteString(strings.Repeat("-", width))
		}
	}
	_, err := fmt.Fprintln(out, b.String())
	return err
}

func sanitizeCell(value string) string {
	replacer := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ")
	return strings.TrimSpace(replacer.Replace(value))
}
