// Command entdb-server is the Go reimplementation of the EntDB
// gRPC server (tracking issue #407). Wave-1 wiring binds a gRPC
// server (every RPC still returns codes.Unimplemented), opens the
// per-tenant SQLite + globalstore handles, and starts the WAL
// applier in a background goroutine. Real RPC handlers land one PR
// at a time as Wave-2 issues are completed.
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/testseed"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

func main() {
	addr := flag.String("addr", ":50051", "gRPC bind address (host:port)")
	dataDir := flag.String("data-dir", "", "directory for per-tenant SQLite + global.db")
	walBackend := flag.String("wal-backend", "memory", "WAL backend: memory (only one supported in Wave 1)")
	walTopic := flag.String("wal-topic", "entdb-wal", "WAL topic name")
	walGroup := flag.String("wal-group", "entdb-applier", "WAL consumer group id")
	// --seed-tenant is a test-only flag honoured by the cross-impl
	// contract harness (docs/go-port/shared/test-harness.md). When set,
	// the binary pre-creates a tenant + the actors / nodes the chosen
	// --seed-profile expects to find. Empty disables seeding.
	seedTenant := flag.String("seed-tenant", "", "test-only: pre-create this tenant before serving (paired with --seed-profile)")
	// --seed-profile selects the fixture shape applied to --seed-tenant.
	//   - "none" (default): no seeding (legacy --seed-tenant without
	//     --seed-profile defaults to "contract" for backwards
	//     compatibility with the Wave-4 harness).
	//   - "contract": User/Task/AssignedTo schema + alice/bob users +
	//     seed node + seed-1 receipt. Matches the cross-impl contract
	//     suite (tests/python/integration/test_grpc_contract.py).
	//   - "e2e": User/Product/Order schema (typeIDs 8001/8002/8003) +
	//     Purchased/PlacedOrder/OrderContains edges + e2e-runner user
	//     as tenant owner. Matches tests/python/e2e/.
	seedProfile := flag.String("seed-profile", "", "test-only: seed profile {none, contract, e2e}; default 'contract' when --seed-tenant is set")
	flag.Parse()

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("entdb-server: listen %s: %v", *addr, err)
	}

	// Resolve --seed-profile. Empty profile + non-empty --seed-tenant
	// defaults to "contract" so the pre-Wave-6 harness invocation
	// (--seed-tenant=acme without --seed-profile) keeps working.
	profile := *seedProfile
	if profile == "" {
		if *seedTenant != "" {
			profile = "contract"
		} else {
			profile = "none"
		}
	}
	switch profile {
	case "none", "contract", "e2e":
		// ok
	default:
		log.Fatalf("entdb-server: invalid --seed-profile %q (want none|contract|e2e)", profile)
	}

	srvOpts := []api.Option{}

	// Schema registry. Populated below when a seed profile selects one
	// so the cross-impl suites' GetSchema/ExecuteAtomic asserts hold;
	// left empty for profile=none (production wiring lands in a later
	// wave once the loader hook is connected to a config source).
	registry := schema.NewRegistry()
	switch profile {
	case "contract":
		if err := testseed.RegisterContractSchema(registry); err != nil {
			log.Fatalf("entdb-server: register contract schema: %v", err)
		}
	case "e2e":
		if err := testseed.RegisterE2ESchema(registry); err != nil {
			log.Fatalf("entdb-server: register e2e schema: %v", err)
		}
	}
	if _, err := registry.Freeze(); err != nil {
		log.Fatalf("entdb-server: freeze registry: %v", err)
	}
	srvOpts = append(srvOpts, api.WithSchemaRegistry(registry))

	// Per-tenant canonical store (optional in Wave-1; if data-dir is
	// unset, RPCs will be served as Unimplemented anyway and the
	// applier won't start).
	var (
		canonical *store.CanonicalStore
		global    *globalstore.GlobalStore
		applier   *apply.Applier
	)
	if *dataDir != "" {
		canonical, err = store.New(store.Options{RootDir: *dataDir, WALMode: true, Registry: registry})
		if err != nil {
			log.Fatalf("entdb-server: open canonical store: %v", err)
		}
		defer func() { _ = canonical.Close() }()
		srvOpts = append(srvOpts, api.WithStore(canonical))

		global, err = globalstore.New(globalstore.Options{DataDir: *dataDir, WALMode: true})
		if err != nil {
			log.Fatalf("entdb-server: open global store: %v", err)
		}
		defer func() { _ = global.Close() }()
		srvOpts = append(srvOpts, api.WithGlobalStore(global))
	}

	// WAL backend wiring. Wave 1 only ships the in-memory backend.
	var walImpl interface {
		wal.Producer
		wal.Consumer
	}
	switch *walBackend {
	case "memory":
		walImpl = wal.NewInMemory(0)
	default:
		log.Fatalf("entdb-server: unsupported wal backend %q (only 'memory' is supported in Wave 1)", *walBackend)
	}
	if err := walImpl.Connect(context.Background()); err != nil {
		log.Fatalf("entdb-server: wal connect: %v", err)
	}
	srvOpts = append(srvOpts, api.WithWALProducer(walImpl), api.WithWALTopic(*walTopic))

	// Applier: only when we have a canonical store.
	if canonical != nil {
		applier, err = apply.New(apply.Options{
			Store:    canonical,
			Global:   global,
			Consumer: walImpl,
			Topic:    *walTopic,
			GroupID:  *walGroup,
		})
		if err != nil {
			log.Fatalf("entdb-server: applier: %v", err)
		}
	}

	srv := grpc.NewServer()
	pb.RegisterEntDBServiceServer(srv, api.New(srvOpts...))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test-only seed: the cross-impl harnesses boot the binary with
	// --seed-tenant <id> --seed-profile <name> and expect the tenant +
	// fixture state to be queryable before the first RPC arrives.
	// Skipped silently when profile=none.
	if profile != "none" {
		if *seedTenant == "" {
			log.Fatalf("entdb-server: --seed-profile=%s requires --seed-tenant", profile)
		}
		if canonical == nil || global == nil {
			log.Fatalf("entdb-server: --seed-profile=%s requires --data-dir", profile)
		}
		switch profile {
		case "contract":
			if err := testseed.SeedTenantContract(ctx, global, canonical, *seedTenant); err != nil {
				log.Fatalf("entdb-server: seed tenant %q (contract): %v", *seedTenant, err)
			}
			log.Printf("entdb-server: seeded tenant %q with contract profile (alice=owner, bob=member, seed node + receipt)", *seedTenant)
		case "e2e":
			if err := testseed.SeedTenantE2E(ctx, global, canonical, *seedTenant); err != nil {
				log.Fatalf("entdb-server: seed tenant %q (e2e): %v", *seedTenant, err)
			}
			log.Printf("entdb-server: seeded tenant %q with e2e profile (e2e-runner=owner, User/Product/Order schema)", *seedTenant)
		}
	}

	// Run the applier in a background goroutine. Halt-on-poison errors
	// surface here; the supervisor (this loop) logs and shuts down.
	applierErr := make(chan error, 1)
	if applier != nil {
		go func() {
			applierErr <- applier.Run(ctx)
		}()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		log.Printf("entdb-server: shutting down")
		if applier != nil {
			applier.Stop()
		}
		cancel()
		srv.GracefulStop()
	}()

	log.Printf("entdb-server: listening on %s (all RPCs return Unimplemented; applier=%v)", *addr, applier != nil)
	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("entdb-server: serve: %v", err)
		}
	}()

	if applier != nil {
		if err := <-applierErr; err != nil && err != context.Canceled {
			log.Printf("entdb-server: applier exited: %v", err)
		}
	} else {
		<-ctx.Done()
	}
}
