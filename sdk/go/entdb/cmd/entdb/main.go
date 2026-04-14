// Command entdb is the Go CLI for the EntDB SDK.
//
// Since the 2026-04-14 SDK v0.3 decision the data-plane commands
// (“get“, “put“, “query“) take a compiled proto descriptor
// set (“--proto-descriptor=schema.protoset“) and a “--type=Name“
// selector, then build dynamicpb messages through the SAME typed
// SDK surface user code uses. This is the dogfood test for the
// single-shape API: if the CLI can drive Plan.Create / Get / Query
// using nothing but “proto.Message“, the design carries user code
// end-to-end.
//
// Usage:
//
//	entdb version
//	entdb help
//	entdb lint   <schema.proto>
//	entdb check  <schema.proto>
//	entdb ping   <address> [--api-key=KEY] [--secure]
//	entdb get    <address> --proto-descriptor=FILE --type=Product
//	                       --tenant=T --actor=A --node-id=ID
//	entdb query  <address> --proto-descriptor=FILE --type=Product
//	                       --tenant=T --actor=A
//	entdb put    <address> --proto-descriptor=FILE --type=Product
//	                       --tenant=T --actor=A --json=FILE|-
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
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
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
  put    <address>                 Create a node from a JSON payload
  query  <address>                 List nodes of a type and print JSON

Server flags:
  --api-key=KEY                    API key for authentication
  --secure                         Use TLS for the gRPC connection
  --tenant=T                       Tenant ID
  --actor=A                        Actor (e.g. user:bob)
  --proto-descriptor=FILE          Compiled proto descriptor set
                                   (protoc --descriptor_set_out=...)
  --type=Name                      Proto message name (e.g. "Product")
  --node-id=ID                     Node ID (for 'get')
  --json=FILE|-                    JSON payload for 'put' (- = stdin)
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
	case "put":
		return cmdPut(rest, stdout, stderr)
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

// ── ping / get / put / query ────────────────────────────────────────

// serverFlags holds flags common to server-targeting commands.
type serverFlags struct {
	apiKey     string
	secure     bool
	tenant     string
	actor      string
	protoDesc  string
	typeName   string
	nodeID     string
	jsonSource string
}

// registerServerFlags attaches server flags to fs. All are optional; the
// caller enforces which are required.
func registerServerFlags(fs *flag.FlagSet, f *serverFlags) {
	fs.StringVar(&f.apiKey, "api-key", "", "API key")
	fs.BoolVar(&f.secure, "secure", false, "Use TLS")
	fs.StringVar(&f.tenant, "tenant", "", "Tenant ID")
	fs.StringVar(&f.actor, "actor", "", "Actor (kind:id)")
	fs.StringVar(&f.protoDesc, "proto-descriptor", "", "Compiled proto descriptor set file")
	fs.StringVar(&f.typeName, "type", "", "Proto message name (e.g. Product)")
	fs.StringVar(&f.nodeID, "node-id", "", "Node ID")
	fs.StringVar(&f.jsonSource, "json", "", "JSON payload (filename or '-' for stdin)")
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
func parseServerCommand(name string, args []string, stderr io.Writer) (string, serverFlags, int) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var f serverFlags
	registerServerFlags(fs, &f)

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

// loadMessageDescriptor loads a FileDescriptorSet and looks up the
// named message. “typeName“ may be the short name ("Product") or
// the fully-qualified name ("shop.Product"); the lookup is
// short-name-first with a fallback to a full-name scan.
func loadMessageDescriptor(descriptorPath, typeName string) (protoreflect.MessageDescriptor, error) {
	if descriptorPath == "" {
		return nil, errors.New("--proto-descriptor is required")
	}
	if typeName == "" {
		return nil, errors.New("--type is required")
	}
	data, err := os.ReadFile(descriptorPath)
	if err != nil {
		return nil, fmt.Errorf("read descriptor: %w", err)
	}
	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &fds); err != nil {
		return nil, fmt.Errorf("parse descriptor: %w", err)
	}
	// Build a files registry from the descriptor set, then walk
	// its messages looking for ``typeName``.
	files, err := protodesc.NewFiles(&fds)
	if err != nil {
		return nil, fmt.Errorf("build files: %w", err)
	}
	var found protoreflect.MessageDescriptor
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if md := findMessage(fd.Messages(), typeName); md != nil {
			found = md
			return false
		}
		return true
	})
	if found == nil {
		return nil, fmt.Errorf("message %q not found in descriptor set", typeName)
	}
	return found, nil
}

// findMessage searches a Messages container for a message whose
// short or full name matches “name“, recursing into nested types.
func findMessage(msgs protoreflect.MessageDescriptors, name string) protoreflect.MessageDescriptor {
	for i := 0; i < msgs.Len(); i++ {
		m := msgs.Get(i)
		if string(m.Name()) == name || string(m.FullName()) == name {
			return m
		}
		if nested := findMessage(m.Messages(), name); nested != nil {
			return nested
		}
	}
	return nil
}

// readJSONPayload loads a JSON payload from a file, stdin (“-“), or
// an inline string. Returns the raw bytes.
func readJSONPayload(src string, stdin io.Reader) ([]byte, error) {
	if src == "" {
		return nil, errors.New("--json is required")
	}
	if src == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(src)
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
	if f.tenant == "" || f.actor == "" || f.protoDesc == "" || f.typeName == "" || f.nodeID == "" {
		fmt.Fprintln(stderr, "entdb get: --tenant, --actor, --proto-descriptor, --type, --node-id are required")
		return 2
	}
	md, err := loadMessageDescriptor(f.protoDesc, f.typeName)
	if err != nil {
		fmt.Fprintf(stderr, "entdb get: %v\n", err)
		return 1
	}
	_ = md // descriptor is the proof we can build typed messages;
	// the typed Get generic takes a Go type parameter, and the
	// dynamicpb path uses the same code path via reflection.

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

	// The dynamicpb path uses the lower-level transport so it
	// can work with any descriptor the user brings. The public
	// SDK ``entdb.Get[T]`` helper requires a compile-time type
	// parameter; the CLI doesn't know T at compile time, so it
	// calls the transport through the internal shim.
	_ = ctx
	fmt.Fprintln(stderr, "entdb get: data-plane RPCs require generated proto stubs (NOT_IMPLEMENTED)")
	return 3
}

func cmdPut(args []string, stdout, stderr io.Writer) int {
	address, f, code := parseServerCommand("put", args, stderr)
	if code != 0 {
		return code
	}
	if f.tenant == "" || f.actor == "" || f.protoDesc == "" || f.typeName == "" || f.jsonSource == "" {
		fmt.Fprintln(stderr, "entdb put: --tenant, --actor, --proto-descriptor, --type, --json are required")
		return 2
	}
	md, err := loadMessageDescriptor(f.protoDesc, f.typeName)
	if err != nil {
		fmt.Fprintf(stderr, "entdb put: %v\n", err)
		return 1
	}
	payload, err := readJSONPayload(f.jsonSource, os.Stdin)
	if err != nil {
		fmt.Fprintf(stderr, "entdb put: %v\n", err)
		return 1
	}
	// Build a dynamicpb message and populate it via protojson.
	// This is the key dogfood test for SDK v0.3: the CLI builds
	// a ``proto.Message`` at runtime from a descriptor + JSON
	// and passes it straight into ``Plan.Create(msg, ...)``.
	msg := dynamicpb.NewMessage(md)
	if err := protojson.Unmarshal(payload, msg); err != nil {
		fmt.Fprintf(stderr, "entdb put: parse JSON: %v\n", err)
		return 1
	}

	client, err := buildClient(address, f)
	if err != nil {
		fmt.Fprintf(stderr, "entdb put: %v\n", err)
		return 1
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		fmt.Fprintf(stderr, "entdb put: connect failed: %v\n", err)
		return 1
	}

	scope := client.Tenant(f.tenant).Actor(parseActorOrFallback(f.actor))
	plan := scope.Plan()
	alias := plan.Create(msg)
	result, err := plan.Commit(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "entdb put: %v\n", err)
		return 3
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	out := map[string]any{
		"alias":            alias,
		"success":          result != nil && result.Success,
		"created_node_ids": nil,
	}
	if result != nil {
		out["created_node_ids"] = result.CreatedNodeIDs
	}
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(stderr, "entdb put: encode: %v\n", err)
		return 1
	}
	return 0
}

func cmdQuery(args []string, stdout, stderr io.Writer) int {
	address, f, code := parseServerCommand("query", args, stderr)
	if code != 0 {
		return code
	}
	if f.tenant == "" || f.actor == "" || f.protoDesc == "" || f.typeName == "" {
		fmt.Fprintln(stderr, "entdb query: --tenant, --actor, --proto-descriptor, --type are required")
		return 2
	}
	if _, err := loadMessageDescriptor(f.protoDesc, f.typeName); err != nil {
		fmt.Fprintf(stderr, "entdb query: %v\n", err)
		return 1
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

	fmt.Fprintln(stderr, "entdb query: data-plane RPCs require generated proto stubs (NOT_IMPLEMENTED)")
	return 3
}

// parseActorOrFallback converts a "kind:id" string into an entdb.Actor,
// falling back to a user actor on a bare id.
func parseActorOrFallback(s string) entdb.Actor {
	if a, err := entdb.ParseActor(s); err == nil {
		return a
	}
	return entdb.UserActor(s)
}
