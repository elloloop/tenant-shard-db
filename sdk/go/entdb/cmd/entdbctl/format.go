package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// printJSON marshals a proto message as canonical JSON.
// The single-message form is sufficient because every wired
// RPC returns one Response message — repeated fields are
// nested inside it.
func printJSON(w io.Writer, msg proto.Message) error {
	mo := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: false,
		UseProtoNames:   true,
	}
	b, err := mo.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// table is a tiny wrapper around text/tabwriter so the
// per-command code stays declarative — header row, then
// one Row() call per record.
type table struct {
	tw      *tabwriter.Writer
	headers []string
}

func newTable(w io.Writer, headers ...string) *table {
	t := &table{
		tw:      tabwriter.NewWriter(w, 0, 0, 2, ' ', 0),
		headers: headers,
	}
	fmt.Fprintln(t.tw, strings.Join(headers, "\t"))
	return t
}

func (t *table) Row(cells ...string) {
	// Pad short rows with empty cells so the columns line up
	// even when a sparse response leaves trailing fields blank.
	for len(cells) < len(t.headers) {
		cells = append(cells, "")
	}
	fmt.Fprintln(t.tw, strings.Join(cells, "\t"))
}

func (t *table) Flush() error {
	return t.tw.Flush()
}

// truncate trims s to n runes, appending "…" when it had to cut.
// Used for payload previews in tabular output — the JSON form
// is always available via --format=json for the full text.
func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// payloadPreview json-encodes a structpb-style map and
// truncates it for tabular display.
func payloadPreview(payload map[string]any, n int) string {
	if len(payload) == 0 {
		return ""
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("<unmarshalable: %v>", err)
	}
	return truncate(string(b), n)
}

// writers shared by every subcommand. Tests swap them
// for bytes.Buffers via setOutputs.
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

func setOutputs(out, err io.Writer) func() {
	prevOut, prevErr := stdout, stderr
	stdout, stderr = out, err
	return func() { stdout, stderr = prevOut, prevErr }
}
