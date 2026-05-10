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
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

func main() {
	addr := flag.String("addr", ":50051", "gRPC bind address (host:port)")
	dataDir := flag.String("data-dir", "", "directory for per-tenant SQLite + global.db")
	walBackend := flag.String("wal-backend", "memory", "WAL backend: memory (only one supported in Wave 1)")
	walTopic := flag.String("wal-topic", "entdb-wal", "WAL topic name")
	walGroup := flag.String("wal-group", "entdb-applier", "WAL consumer group id")
	flag.Parse()

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("entdb-server: listen %s: %v", *addr, err)
	}

	srvOpts := []api.Option{}

	// Per-tenant canonical store (optional in Wave-1; if data-dir is
	// unset, RPCs will be served as Unimplemented anyway and the
	// applier won't start).
	var (
		canonical *store.CanonicalStore
		global    *globalstore.GlobalStore
		applier   *apply.Applier
	)
	if *dataDir != "" {
		canonical, err = store.New(store.Options{RootDir: *dataDir, WALMode: true})
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
	srvOpts = append(srvOpts, api.WithWALProducer(walImpl))

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
