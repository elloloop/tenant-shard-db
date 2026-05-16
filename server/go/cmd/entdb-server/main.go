// Command entdb-server is the Go reimplementation of the EntDB
// gRPC server (tracking issue #407). Wave-1 wiring binds a gRPC
// server, opens the per-tenant SQLite + globalstore handles, and
// starts the WAL applier in a background goroutine before accepting
// writes.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"google.golang.org/grpc"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/audit"
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
	tlsCert := flag.String("tls-cert", "", "server TLS certificate PEM file")
	tlsKey := flag.String("tls-key", "", "server TLS private key PEM file")
	tlsCA := flag.String("tls-ca", "", "client CA PEM file for mTLS verification")
	tlsMinVersion := flag.String("tls-min-version", "1.3", "minimum TLS version: 1.3 | 1.2")
	requireTLS := flag.Bool("require-tls", false, "refuse to start unless TLS is configured")
	requireClientCert := flag.Bool("require-client-cert", false, "require and verify client certificates (mTLS; requires --tls-ca)")
	walBackend := flag.String("wal-backend", "memory", "WAL backend: memory | kafka")
	walTopic := flag.String("wal-topic", "entdb-wal", "WAL topic name")
	walGroup := flag.String("wal-group", "entdb-applier", "WAL consumer group id")
	// Kafka/Redpanda-specific knobs. Defaults match
	// server/python/entdb_server/wal/kafka.py + config.py (KAFKA_BROKERS
	// etc.) so the cross-impl e2e stack can swap targets without
	// re-jiggering compose env-vars.
	walBrokers := flag.String("wal-brokers", "localhost:9092", "comma-separated Kafka bootstrap brokers (used when --wal-backend=kafka)")
	archiveEnabled := flag.Bool("archive-enabled", false, "enable S3 Object Lock WAL archive sidecar (requires --wal-backend=kafka)")
	archiveBucket := flag.String("archive-bucket", "", "S3 bucket for immutable WAL archives")
	archiveRegion := flag.String("archive-region", "", "AWS region for --archive-bucket")
	archiveGroup := flag.String("archive-group", "entdb-wal-archive", "WAL consumer group id for the archive sidecar")
	archiveRetentionDays := flag.Int("archive-retention-days", 2557, "S3 Object Lock COMPLIANCE retention window in days")
	archiveKMSKeyID := flag.String("archive-kms-key-id", "", "optional AWS KMS key id for archive object SSE-KMS")
	archiveBatchSize := flag.Int("archive-batch-size", 128, "maximum WAL records per archive poll")
	archiveBatchBytes := flag.Int("archive-batch-bytes", 10<<20, "approximate maximum uncompressed bytes per archive object")
	archivePollTimeout := flag.Duration("archive-poll-timeout", time.Second, "how long the archive sidecar polls for WAL records")
	archiveRetryBackoff := flag.Duration("archive-retry-backoff", 5*time.Second, "retry backoff after archive poll/write failures")
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

	if strings.TrimSpace(*dataDir) == "" {
		log.Fatalf("entdb-server: --data-dir is required")
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

	registry, err := schemaRegistryForProfile(profile)
	if err != nil {
		log.Fatalf("entdb-server: schema registry: %v", err)
	}
	if registry != nil {
		srvOpts = append(srvOpts, api.WithSchemaRegistry(registry))
	}

	canonical, err := store.New(store.Options{RootDir: *dataDir, WALMode: true, Registry: registry})
	if err != nil {
		log.Fatalf("entdb-server: open canonical store: %v", err)
	}
	defer func() { _ = canonical.Close() }()
	srvOpts = append(srvOpts, api.WithStore(canonical))

	global, err := globalstore.New(globalstore.Options{DataDir: *dataDir, WALMode: true})
	if err != nil {
		log.Fatalf("entdb-server: open global store: %v", err)
	}
	defer func() { _ = global.Close() }()
	srvOpts = append(srvOpts, api.WithGlobalStore(global))

	// WAL backend wiring. memory: in-process, lost on restart (dev /
	// short-running tests). kafka: Kafka/Redpanda; production-grade,
	// survives docker compose restart, exact-parity with Python's
	// wal/kafka.py.
	var walImpl interface {
		wal.Producer
		wal.Consumer
	}
	switch *walBackend {
	case "memory":
		walImpl = wal.NewInMemory(0)
	case "kafka":
		brokers := splitBrokers(*walBrokers)
		if len(brokers) == 0 {
			log.Fatalf("entdb-server: --wal-backend=kafka requires --wal-brokers")
		}
		walImpl = wal.NewKafka(wal.DefaultKafkaConfig(brokers))
	default:
		log.Fatalf("entdb-server: unsupported wal backend %q (want memory|kafka)", *walBackend)
	}
	if err := walImpl.Connect(context.Background()); err != nil {
		log.Fatalf("entdb-server: wal connect: %v", err)
	}
	srvOpts = append(srvOpts, api.WithWALProducer(walImpl), api.WithWALTopic(*walTopic))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var archiver *audit.Archiver
	if *archiveEnabled {
		if *walBackend != "kafka" {
			log.Fatalf("entdb-server: --archive-enabled requires --wal-backend=kafka")
		}
		if strings.TrimSpace(*archiveBucket) == "" {
			log.Fatalf("entdb-server: --archive-enabled requires --archive-bucket")
		}
		if strings.TrimSpace(*archiveRegion) == "" {
			log.Fatalf("entdb-server: --archive-enabled requires --archive-region")
		}
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(strings.TrimSpace(*archiveRegion)))
		if err != nil {
			log.Fatalf("entdb-server: load AWS config for archive: %v", err)
		}
		archiveStore := audit.NewS3ObjectLockStore(
			s3.NewFromConfig(awsCfg),
			strings.TrimSpace(*archiveBucket),
			strings.TrimSpace(*archiveKMSKeyID),
		)
		archiver, err = audit.NewArchiver(audit.Options{
			Consumer:        walImpl,
			Store:           archiveStore,
			Topic:           *walTopic,
			GroupID:         *archiveGroup,
			RetentionDays:   *archiveRetentionDays,
			BatchSize:       *archiveBatchSize,
			BatchBytes:      *archiveBatchBytes,
			PollTimeout:     *archivePollTimeout,
			RetryBackoff:    *archiveRetryBackoff,
			LegalHoldFunc:   global.IsLegalHoldSet,
			SkipVerifyOnRun: true,
		})
		if err != nil {
			log.Fatalf("entdb-server: archive sidecar: %v", err)
		}
		if err := archiver.Verify(ctx); err != nil {
			log.Fatalf("entdb-server: archive object lock verification: %v", err)
		}
		log.Printf("entdb-server: S3 Object Lock archive enabled (bucket=%s group=%s)", strings.TrimSpace(*archiveBucket), *archiveGroup)
	}

	applier, err := apply.New(apply.Options{
		Store:    canonical,
		Global:   global,
		Consumer: walImpl,
		Topic:    *walTopic,
		GroupID:  *walGroup,
	})
	if err != nil {
		log.Fatalf("entdb-server: applier: %v", err)
	}

	// Run the applier before the gRPC server starts accepting writes.
	// Halt-on-poison errors surface here; the supervisor logs and
	// starts shutdown.
	applierErr := make(chan error, 1)
	go func() {
		applierErr <- applier.Run(ctx)
	}()
	archiveErr := make(chan error, 1)
	if archiver != nil {
		go func() {
			archiveErr <- archiver.Run(ctx)
		}()
	}

	// Test-only seed: the cross-impl harnesses boot the binary with
	// --seed-tenant <id> --seed-profile <name> and expect the tenant +
	// fixture state to be queryable before the first RPC arrives.
	// Skipped silently when profile=none.
	if profile != "none" {
		if *seedTenant == "" {
			log.Fatalf("entdb-server: --seed-profile=%s requires --seed-tenant", profile)
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

	grpcServerOpts, tlsEnabled, err := grpcServerTLSOptions(serverTLSConfig{
		certFile:          *tlsCert,
		keyFile:           *tlsKey,
		caFile:            *tlsCA,
		minVersion:        *tlsMinVersion,
		requireTLS:        *requireTLS,
		requireClientCert: *requireClientCert,
	})
	if err != nil {
		log.Fatalf("entdb-server: TLS config: %v", err)
	}
	if tlsEnabled {
		log.Printf("entdb-server: TLS enabled (min-version=%s client-auth=%s)", *tlsMinVersion, clientAuthMode(*tlsCA, *requireClientCert))
	} else {
		log.Printf("entdb-server: WARNING: TLS disabled; plaintext gRPC listener is for local/dev use only")
	}

	srv := grpc.NewServer(grpcServerOpts...)
	pb.RegisterEntDBServiceServer(srv, api.New(srvOpts...))

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("entdb-server: listen %s: %v", *addr, err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		log.Printf("entdb-server: shutting down")
		applier.Stop()
		cancel()
		srv.GracefulStop()
	}()

	log.Printf("entdb-server: listening on %s (applier running)", *addr)
	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("entdb-server: serve: %v", err)
		}
	}()

	select {
	case err := <-applierErr:
		if err != nil && err != context.Canceled {
			log.Printf("entdb-server: applier exited: %v", err)
		}
	case err := <-archiveErr:
		if err != nil && err != context.Canceled {
			log.Printf("entdb-server: archive sidecar exited: %v", err)
		}
	}
}

// schemaRegistryForProfile returns nil for profile=none because a nil
// registry is the API server's schema-less mode. An empty frozen
// registry is materially different: QueryNodes rejects every type_id
// as unknown.
func schemaRegistryForProfile(profile string) (*schema.Registry, error) {
	switch profile {
	case "none":
		return nil, nil
	case "contract", "e2e":
		// handled below
	default:
		return nil, fmt.Errorf("invalid profile %q", profile)
	}

	registry := schema.NewRegistry()
	switch profile {
	case "contract":
		if err := testseed.RegisterContractSchema(registry); err != nil {
			return nil, fmt.Errorf("register contract schema: %w", err)
		}
	case "e2e":
		if err := testseed.RegisterE2ESchema(registry); err != nil {
			return nil, fmt.Errorf("register e2e schema: %w", err)
		}
	}
	if _, err := registry.Freeze(); err != nil {
		return nil, fmt.Errorf("freeze registry: %w", err)
	}
	return registry, nil
}

// splitBrokers parses a comma-separated broker list and returns
// non-empty entries. Mirrors the way Python passes brokers as a single
// string via KAFKA_BROKERS.
func splitBrokers(s string) []string {
	out := []string{}
	for _, b := range strings.Split(s, ",") {
		b = strings.TrimSpace(b)
		if b != "" {
			out = append(out, b)
		}
	}
	return out
}
