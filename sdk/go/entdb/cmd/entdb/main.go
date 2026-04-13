// Command entdb is the Go CLI for the EntDB SDK.
//
// It mirrors the most useful subset of the Python entdb CLI and also
// exposes ping/get/query against a live EntDB server via the Go SDK.
//
// Usage:
//
//	entdb version
//	entdb help
//	entdb lint   <schema.proto>
//	entdb check  <schema.proto>
//	entdb ping   <address> [--api-key=KEY] [--secure]
//	entdb get    <address> --tenant=T --actor=A --type-id=N --node-id=ID
//	entdb query  <address> --tenant=T --actor=A --type-id=N
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/elloloop/tenant-shard-db/sdk/go/entdb"
)

const helpText = `entdb — Go CLI for EntDB

Usage:
  entdb <command> [flags] [args]

Schema commands:
  version                          Print version + git SHA
  help                             Print this help
  lint   <schema.proto>            Validate proto has at least one (entdb.node)
  check  <schema.proto>            Compile proto via protoc and list node fields

Server commands:
  ping   <address>                 Connect and verify server responds
  get    <address>                 Fetch a node by ID and print JSON
  query  <address>                 List nodes of a type and print JSON

Server flags:
  --api-key=KEY                    API key for authentication
  --secure                         Use TLS for the gRPC connection
  --tenant=T                       Tenant ID
  --actor=A                        Actor (e.g. user:bob)
  --type-id=N                      Node type ID (int)
  --node-id=ID                     Node ID (for 'get')
`

// run is the testable CLI entry point. It returns the exit code and
// does not call os.Exit itself.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, helpText)
		return 2
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "version", "-v", "--version":
		printVersion(stdout)
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(stdout, helpText)
		return 0
	case "lint":
		return cmdLint(rest, stdout, stderr)
	case "check":
		return cmdCheck(rest, stdout, stderr)
	case "ping":
		return cmdPing(rest, stdout, stderr)
	case "get":
		return cmdGet(rest, stdout, stderr)
	case "query":
		return cmdQuery(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "entdb: unknown command %q\n\n", cmd)
		fmt.Fprint(stderr, helpText)
		return 2
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// ── lint / check ────────────────────────────────────────────────────

// requireProtoc returns an error if protoc is not on PATH.
func requireProtoc() error {
	if _, err := exec.LookPath("protoc"); err != nil {
		return errors.New("protoc not found on PATH — install Protocol Buffers compiler (https://grpc.io/docs/protoc-installation/)")
	}
	return nil
}

// readSchemaFile returns the contents of path, or an error.
func readSchemaFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// countOccurrences counts non-overlapping occurrences of needle in s.
func countOccurrences(s, needle string) int {
	return strings.Count(s, needle)
}

func cmdLint(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "entdb lint: missing <schema.proto>")
		return 2
	}
	path := fs.Arg(0)

	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(stderr, "entdb lint: %v\n", err)
		return 1
	}

	if err := requireProtoc(); err != nil {
		fmt.Fprintf(stderr, "entdb lint: %v\n", err)
		return 1
	}

	contents, err := readSchemaFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "entdb lint: %v\n", err)
		return 1
	}

	nodes := countOccurrences(contents, "(entdb.node)")
	edges := countOccurrences(contents, "(entdb.edge)")

	if nodes == 0 {
		fmt.Fprintln(stderr, "entdb lint: schema has no (entdb.node) annotations")
		return 1
	}

	// Ask protoc to parse the file to surface syntax errors.
	out, err := exec.Command("protoc", "-o/dev/null", path).CombinedOutput()
	if err != nil {
		fmt.Fprintf(stderr, "entdb lint: protoc failed:\n%s", out)
		return 1
	}

	fmt.Fprintf(stdout, "Lint passed: %d node type(s), %d edge type(s)\n", nodes, edges)
	return 0
}

func cmdCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "entdb check: missing <schema.proto>")
		return 2
	}
	path := fs.Arg(0)

	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(stderr, "entdb check: %v\n", err)
		return 1
	}

	if err := requireProtoc(); err != nil {
		fmt.Fprintf(stderr, "entdb check: %v\n", err)
		return 1
	}

	out, err := exec.Command("protoc", "-o/dev/null", path).CombinedOutput()
	if err != nil {
		fmt.Fprintf(stderr, "entdb check: protoc failed:\n%s", out)
		return 1
	}

	contents, err := readSchemaFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "entdb check: %v\n", err)
		return 1
	}
	summarizeFields(contents, stdout)
	return 0
}

// summarizeFields prints a best-effort summary of messages and their
// field_id = type pairs. It uses line-based scanning, not a full parser.
func summarizeFields(contents string, w io.Writer) {
	lines := strings.Split(contents, "\n")
	var currentMsg string
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "message ") {
			// message Foo {
			name := strings.TrimPrefix(line, "message ")
			name = strings.TrimSuffix(name, "{")
			currentMsg = strings.TrimSpace(name)
			fmt.Fprintf(w, "message %s\n", currentMsg)
			continue
		}
		if strings.HasPrefix(line, "}") {
			currentMsg = ""
			continue
		}
		if currentMsg == "" {
			continue
		}
		// crude "type name = id;" detection
		eq := strings.Index(line, "=")
		semi := strings.Index(line, ";")
		if eq < 0 || semi < 0 || semi < eq {
			continue
		}
		lhs := strings.Fields(strings.TrimSpace(line[:eq]))
		if len(lhs) < 2 {
			continue
		}
		typ := lhs[len(lhs)-2]
		name := lhs[len(lhs)-1]
		id := strings.TrimSpace(line[eq+1 : semi])
		fmt.Fprintf(w, "  %s  %s = %s\n", typ, name, id)
	}
}

// ── ping / get / query ──────────────────────────────────────────────

// serverFlags holds flags common to server-targeting commands.
type serverFlags struct {
	apiKey string
	secure bool
	tenant string
	actor  string
	typeID int
	nodeID string
}

// registerServerFlags attaches server flags to fs. All are optional; the
// caller enforces which are required.
func registerServerFlags(fs *flag.FlagSet, f *serverFlags) {
	fs.StringVar(&f.apiKey, "api-key", "", "API key")
	fs.BoolVar(&f.secure, "secure", false, "Use TLS")
	fs.StringVar(&f.tenant, "tenant", "", "Tenant ID")
	fs.StringVar(&f.actor, "actor", "", "Actor (kind:id)")
	fs.IntVar(&f.typeID, "type-id", 0, "Node type ID")
	fs.StringVar(&f.nodeID, "node-id", "", "Node ID")
}

// buildClient constructs a DbClient from flags.
func buildClient(address string, f serverFlags) (*entdb.DbClient, error) {
	opts := []entdb.ClientOption{}
	if f.secure {
		opts = append(opts, entdb.WithSecure())
	}
	if f.apiKey != "" {
		opts = append(opts, entdb.WithAPIKey(f.apiKey))
	}
	return entdb.NewClient(address, opts...)
}

// parseServerCommand parses a command that takes <address> plus server flags.
// The address may appear anywhere in args; flags may come before or after it.
// Returns address, parsed flags, and an exit code (0 on success).
func parseServerCommand(name string, args []string, stderr io.Writer) (string, serverFlags, int) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var f serverFlags
	registerServerFlags(fs, &f)

	// flag.Parse stops at the first non-flag argument. To allow the
	// positional <address> to appear anywhere, we parse iteratively and
	// collect non-flag tokens as positionals.
	var positionals []string
	remaining := args
	for len(remaining) > 0 {
		if err := fs.Parse(remaining); err != nil {
			return "", f, 2
		}
		if fs.NArg() == 0 {
			break
		}
		positionals = append(positionals, fs.Arg(0))
		remaining = fs.Args()[1:]
	}

	if len(positionals) < 1 {
		fmt.Fprintf(stderr, "entdb %s: missing <address>\n", name)
		return "", f, 2
	}
	return positionals[0], f, 0
}

func cmdPing(args []string, stdout, stderr io.Writer) int {
	address, f, code := parseServerCommand("ping", args, stderr)
	if code != 0 {
		return code
	}

	client, err := buildClient(address, f)
	if err != nil {
		fmt.Fprintf(stderr, "entdb ping: %v\n", err)
		return 1
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		fmt.Fprintf(stderr, "entdb ping: connect failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "OK %s (secure=%t)\n", address, f.secure)
	return 0
}

func cmdGet(args []string, stdout, stderr io.Writer) int {
	address, f, code := parseServerCommand("get", args, stderr)
	if code != 0 {
		return code
	}
	if f.tenant == "" || f.actor == "" || f.typeID == 0 || f.nodeID == "" {
		fmt.Fprintln(stderr, "entdb get: --tenant, --actor, --type-id, --node-id are required")
		return 2
	}

	client, err := buildClient(address, f)
	if err != nil {
		fmt.Fprintf(stderr, "entdb get: %v\n", err)
		return 1
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		fmt.Fprintf(stderr, "entdb get: connect failed: %v\n", err)
		return 1
	}

	node, err := client.Get(ctx, f.tenant, f.actor, f.typeID, f.nodeID)
	if err != nil {
		fmt.Fprintf(stderr, "entdb get: %v\n", err)
		return 3 // NOT_IMPLEMENTED / server error
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(node); err != nil {
		fmt.Fprintf(stderr, "entdb get: encode: %v\n", err)
		return 1
	}
	return 0
}

func cmdQuery(args []string, stdout, stderr io.Writer) int {
	address, f, code := parseServerCommand("query", args, stderr)
	if code != 0 {
		return code
	}
	if f.tenant == "" || f.actor == "" || f.typeID == 0 {
		fmt.Fprintln(stderr, "entdb query: --tenant, --actor, --type-id are required")
		return 2
	}

	client, err := buildClient(address, f)
	if err != nil {
		fmt.Fprintf(stderr, "entdb query: %v\n", err)
		return 1
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		fmt.Fprintf(stderr, "entdb query: connect failed: %v\n", err)
		return 1
	}

	nodes, err := client.Query(ctx, f.tenant, f.actor, f.typeID, nil)
	if err != nil {
		fmt.Fprintf(stderr, "entdb query: %v\n", err)
		return 3 // NOT_IMPLEMENTED / server error
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(nodes); err != nil {
		fmt.Fprintf(stderr, "entdb query: encode: %v\n", err)
		return 1
	}
	return 0
}
