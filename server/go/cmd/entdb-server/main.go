// Command entdb-server is the canonical EntDB gRPC server. It binds
// a gRPC server, opens the per-tenant SQLite + globalstore handles,
// and starts the WAL applier in a background goroutine before
// accepting writes.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/audit"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	entcrypto "github.com/elloloop/tenant-shard-db/server/go/internal/crypto"
	"github.com/elloloop/tenant-shard-db/server/go/internal/gdpr"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/testseed"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

func main() {
	addr := flag.String("addr", ":50051", "gRPC bind address (host:port)")
	metricsAddr := flag.String("metrics-addr", "", "Prometheus scrape bind address (host:port); empty disables the /metrics endpoint")
	dataDir := flag.String("data-dir", "", "directory for per-tenant SQLite + global.db")
	tlsCert := flag.String("tls-cert", "", "server TLS certificate PEM file")
	tlsKey := flag.String("tls-key", "", "server TLS private key PEM file")
	tlsCA := flag.String("tls-ca", "", "client CA PEM file for mTLS verification")
	tlsMinVersion := flag.String("tls-min-version", "1.3", "minimum TLS version: 1.3 | 1.2")
	requireTLS := flag.Bool("require-tls", false, "refuse to start unless TLS is configured")
	requireClientCert := flag.Bool("require-client-cert", false, "require and verify client certificates (mTLS; requires --tls-ca)")
	oauthProvider := flag.String("oauth-provider", "", "OIDC provider preset: google | microsoft | okta (microsoft/okta also need --oauth-issuer)")
	oauthIssuer := flag.String("oauth-issuer", "", "OIDC issuer URL; expected JWT 'iss' and the discovery base when --jwks-url is unset")
	oauthJWKSURL := flag.String("jwks-url", "", "explicit JWKS endpoint; when set, OIDC discovery is skipped")
	oauthAudience := flag.String("oauth-audience", "", "expected JWT 'aud' (your OAuth client/app id); empty disables the audience check (dev only)")
	apiKeyAuth := flag.Bool("api-key-auth", false, "enable persistent argon2id-hashed API-key authentication backed by global.db (x-api-key header)")
	kmsProvider := flag.String("kms-provider", "", "encryption master-key provider: file | aws | gcp | azure | vault")
	kmsKeyID := flag.String("kms-key-id", "", "master-key provider identifier; file provider accepts path or env:NAME")
	encryptionRequired := flag.Bool("encryption-required", false, "require encrypted global.db and tenant SQLite files")
	readPoolSize := flag.Int("read-pool-size", 1, "per-tenant read-only SQLite connection pool size (issue #137 / ADR-026). OFF BY DEFAULT (1 = single shared connection, the proven legacy behaviour) — landed dark pending idle-tenant eviction (canonical-store OQ-2: each read connection adds an FD + page cache per active tenant, with no eviction). Opt IN by setting >1 to let same-tenant reads run concurrently against WAL snapshots instead of serializing behind the applier. The post-COMMIT offset-broadcast fix (ADR-026 condition 1) is always active regardless of this value.")
	gdprWorkerEnabled := flag.Bool("gdpr-worker-enabled", true, "run GDPR deletion_queue worker that performs due deletes and crypto-shred")
	gdprWorkerInterval := flag.Duration("gdpr-worker-interval", time.Minute, "how often the GDPR worker scans for due deletions")
	legalHoldLiftWorkerEnabled := flag.Bool("legal-hold-lift-worker-enabled", true, "run the durable legal_hold_lift_queue worker that lifts S3 Object Lock legal hold on a released tenant's already-archived objects (EPIC #511 Gap 1); only does work when -archive-enabled")
	legalHoldLiftWorkerInterval := flag.Duration("legal-hold-lift-worker-interval", time.Minute, "how often the legal-hold-lift worker drains the pending-lift queue")
	cryptoShredDeleteFiles := flag.Bool("crypto-shred-delete-files", false, "delete tenant .db/-wal/-shm files after key shred")
	walBackend := flag.String("wal-backend", "memory", "WAL backend: memory | kafka | kinesis | pubsub | sqs | servicebus | eventhubs")
	walTopic := flag.String("wal-topic", "entdb-wal", "WAL topic name")
	walGroup := flag.String("wal-group", "entdb-applier", "WAL consumer group id")
	// --apply-concurrency controls cross-tenant parallel WAL apply
	// (#140 / PERF-4, ADR-027). Within a single poll batch the applier
	// may apply records for DISTINCT tenant route keys in parallel;
	// records for one tenant are always serial in offset order and
	// offsets commit strictly in batch order, so this knob never
	// weakens per-tenant ordering, the single-writer-per-tenant
	// invariant (ADR-016), or the gap-free contiguous-prefix commit.
	// LANDED DARK (ADR-027): default 1 = strictly serial, the unchanged
	// pre-#140 behaviour. Operators opt into parallelism after a
	// staging soak by setting 0 (-> runtime.GOMAXPROCS) or an explicit
	// N>1. This is both the rollout posture and the no-redeploy
	// kill-switch.
	applyConcurrency := flag.Int("apply-concurrency", 1,
		"max distinct tenants applied in parallel per WAL poll batch "+
			"(1 = strictly serial / pre-#140, the default; 0 = runtime.GOMAXPROCS; N>1 = opt-in parallel)")
	// Kafka/Redpanda-specific knobs. Defaults match the legacy
	// KAFKA_BROKERS env-var convention so the cross-impl e2e stack
	// can swap targets without re-jiggering compose env-vars.
	walBrokers := flag.String("wal-brokers", "localhost:9092", "comma-separated Kafka bootstrap brokers (used when --wal-backend=kafka)")
	// Cloud-native backend knobs (ADR-005 / EPIC #518). Each backend
	// follows the existing Kafka pattern: backend-specific flags,
	// shared --wal-topic / --wal-group. Unset flags are only an error
	// if the matching --wal-backend is selected.
	//
	// AWS Kinesis.
	walKinesisStream := flag.String("wal-kinesis-stream", "", "Kinesis stream name (used when --wal-backend=kinesis; defaults to --wal-topic)")
	walAWSRegion := flag.String("wal-aws-region", "", "AWS region for Kinesis/SQS backends")
	walAWSEndpoint := flag.String("wal-aws-endpoint", "", "override AWS endpoint URL (LocalStack/testing) for Kinesis/SQS backends")
	walKinesisIterator := flag.String("wal-kinesis-iterator", "TRIM_HORIZON", "Kinesis shard iterator type: TRIM_HORIZON | LATEST")
	// AWS SQS (FIFO).
	walSQSQueueURL := flag.String("wal-sqs-queue-url", "", "SQS FIFO queue URL (used when --wal-backend=sqs; must end in .fifo)")
	// GCP Pub/Sub.
	walPubSubProject := flag.String("wal-pubsub-project", "", "GCP project id (used when --wal-backend=pubsub)")
	walPubSubTopic := flag.String("wal-pubsub-topic", "", "Pub/Sub topic id (used when --wal-backend=pubsub; defaults to --wal-topic)")
	walPubSubSub := flag.String("wal-pubsub-subscription", "", "Pub/Sub subscription id (used when --wal-backend=pubsub; defaults to --wal-group)")
	walPubSubEndpoint := flag.String("wal-pubsub-endpoint", "", "override Pub/Sub endpoint (emulator/testing)")
	// Azure Service Bus / Event Hubs.
	walAzureConnStr := flag.String("wal-azure-connection-string", "", "Azure Service Bus / Event Hubs namespace connection string")
	walServiceBusQueue := flag.String("wal-servicebus-queue", "", "Azure Service Bus session-enabled queue name (used when --wal-backend=servicebus; defaults to --wal-topic)")
	walEventHubsName := flag.String("wal-eventhubs-name", "", "Azure Event Hub name (used when --wal-backend=eventhubs; defaults to --wal-topic)")
	walEventHubsGroup := flag.String("wal-eventhubs-consumer-group", "", "Azure Event Hubs consumer group (used when --wal-backend=eventhubs; defaults to $Default)")
	archiveEnabled := flag.Bool("archive-enabled", false, "enable S3 Object Lock WAL archive sidecar (requires --wal-backend=kafka)")
	archiveBucket := flag.String("archive-bucket", "", "S3 bucket for immutable WAL archives")
	archiveRegion := flag.String("archive-region", "", "AWS region for --archive-bucket")
	archiveGroup := flag.String("archive-group", "entdb-wal-archive", "WAL consumer group id for the archive sidecar")
	archiveRetentionDays := flag.Int("archive-retention-days", 2557, "S3 Object Lock COMPLIANCE retention window in days")
	archiveKMSKeyID := flag.String("archive-kms-key-id", "", "optional AWS KMS key id for archive object SSE-KMS")
	archiveS3PathStyle := flag.Bool("archive-s3-path-style", false, "use S3 path-style addressing for the archive bucket (required for MinIO and other non-AWS S3 endpoints)")
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
	//     compatibility with the harness).
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
	// defaults to "contract" so the pre- harness invocation
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var masterKey []byte
	encryptionConfigured := *encryptionRequired ||
		strings.TrimSpace(*kmsProvider) != "" ||
		strings.TrimSpace(*kmsKeyID) != ""
	if encryptionConfigured {
		masterKey, err = entcrypto.LoadMasterKey(ctx, entcrypto.MasterKeyConfig{
			Provider: *kmsProvider,
			KeyID:    *kmsKeyID,
			DataDir:  *dataDir,
		})
		if err != nil {
			log.Fatalf("entdb-server: encryption master key: %v", err)
		}
	}

	global, err := globalstore.New(globalstore.Options{
		DataDir:            *dataDir,
		WALMode:            true,
		EncryptionKey:      masterKey,
		EncryptionRequired: *encryptionRequired,
	})
	if err != nil {
		log.Fatalf("entdb-server: open global store: %v", err)
	}
	defer func() { _ = global.Close() }()
	srvOpts = append(srvOpts, api.WithGlobalStore(global))

	var keyManager *entcrypto.KeyManager
	if len(masterKey) > 0 {
		vault, err := entcrypto.NewTenantKeyVault(ctx, entcrypto.TenantKeyVaultOptions{
			DB:        global.DB(),
			MasterKey: masterKey,
		})
		if err != nil {
			log.Fatalf("entdb-server: tenant key vault: %v", err)
		}
		keyManager, err = entcrypto.NewKeyManager(masterKey, vault)
		if err != nil {
			log.Fatalf("entdb-server: key manager: %v", err)
		}
		log.Printf("entdb-server: encryption-at-rest enabled (kms-provider=%s encryption-required=%v)", effectiveKMSProvider(*kmsProvider), *encryptionRequired)
	} else {
		log.Printf("entdb-server: WARNING: encryption-at-rest disabled; plaintext SQLite files are for local/dev use only")
	}

	canonical, err := store.New(store.Options{
		RootDir:            *dataDir,
		WALMode:            true,
		ReadPoolSize:       *readPoolSize,
		Registry:           registry,
		KeyManager:         keyManager,
		EncryptionRequired: *encryptionRequired,
	})
	if err != nil {
		log.Fatalf("entdb-server: open canonical store: %v", err)
	}
	defer func() { _ = canonical.Close() }()
	srvOpts = append(srvOpts, api.WithStore(canonical))

	// WAL backend wiring. memory: in-process, lost on restart (dev /
	// short-running tests). kafka: Kafka/Redpanda; production-grade,
	// survives docker compose restart. The cloud-native backends
	// (kinesis/pubsub/sqs/servicebus/eventhubs) are the EPIC #518 ports
	// — same wal.Producer + wal.Consumer interface, selected by config,
	// not code (ADR-005).
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
	case "kinesis":
		if strings.TrimSpace(*walAWSRegion) == "" {
			log.Fatalf("entdb-server: --wal-backend=kinesis requires --wal-aws-region")
		}
		stream := firstNonEmpty(*walKinesisStream, *walTopic)
		kcfg := wal.DefaultKinesisConfig(stream, strings.TrimSpace(*walAWSRegion))
		kcfg.EndpointURL = strings.TrimSpace(*walAWSEndpoint)
		if strings.TrimSpace(*walKinesisIterator) != "" {
			kcfg.IteratorType = strings.TrimSpace(*walKinesisIterator)
		}
		walImpl = wal.NewKinesis(kcfg)
	case "sqs":
		if strings.TrimSpace(*walSQSQueueURL) == "" {
			log.Fatalf("entdb-server: --wal-backend=sqs requires --wal-sqs-queue-url")
		}
		if strings.TrimSpace(*walAWSRegion) == "" {
			log.Fatalf("entdb-server: --wal-backend=sqs requires --wal-aws-region")
		}
		scfg := wal.DefaultSqsConfig(strings.TrimSpace(*walSQSQueueURL), strings.TrimSpace(*walAWSRegion))
		scfg.EndpointURL = strings.TrimSpace(*walAWSEndpoint)
		walImpl = wal.NewSqs(scfg)
	case "pubsub":
		if strings.TrimSpace(*walPubSubProject) == "" {
			log.Fatalf("entdb-server: --wal-backend=pubsub requires --wal-pubsub-project")
		}
		pcfg := wal.DefaultPubSubConfig(
			strings.TrimSpace(*walPubSubProject),
			firstNonEmpty(*walPubSubTopic, *walTopic),
			firstNonEmpty(*walPubSubSub, *walGroup),
		)
		pcfg.Endpoint = strings.TrimSpace(*walPubSubEndpoint)
		walImpl = wal.NewPubSub(pcfg)
	case "servicebus":
		if strings.TrimSpace(*walAzureConnStr) == "" {
			log.Fatalf("entdb-server: --wal-backend=servicebus requires --wal-azure-connection-string")
		}
		walImpl = wal.NewServiceBus(wal.DefaultServiceBusConfig(
			strings.TrimSpace(*walAzureConnStr),
			firstNonEmpty(*walServiceBusQueue, *walTopic),
		))
	case "eventhubs":
		if strings.TrimSpace(*walAzureConnStr) == "" {
			log.Fatalf("entdb-server: --wal-backend=eventhubs requires --wal-azure-connection-string")
		}
		walImpl = wal.NewEventHubs(wal.DefaultEventHubsConfig(
			strings.TrimSpace(*walAzureConnStr),
			firstNonEmpty(*walEventHubsName, *walTopic),
			strings.TrimSpace(*walEventHubsGroup),
		))
	default:
		log.Fatalf("entdb-server: unsupported wal backend %q (want memory|kafka|kinesis|pubsub|sqs|servicebus|eventhubs)", *walBackend)
	}
	if err := walImpl.Connect(ctx); err != nil {
		log.Fatalf("entdb-server: wal connect: %v", err)
	}
	srvOpts = append(srvOpts, api.WithWALProducer(walImpl), api.WithWALTopic(*walTopic))

	var archiver *audit.Archiver
	var archiveStore *audit.S3ObjectLockStore
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
		s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			if *archiveS3PathStyle {
				o.UsePathStyle = true
			}
			// AWS SDK Go v2 (Jan-2025 default change) computes a CRC32
			// request checksum on every supported op. MinIO and other
			// non-AWS S3 endpoints reject that on legacy
			// Content-MD5-required operations (PutObjectLegalHold ->
			// HTTP 400 MissingContentMD5). Scope checksum calculation to
			// ops that strictly require it so the SDK falls back to the
			// Content-MD5 those endpoints expect; harmless on real AWS.
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		})
		archiveStore = audit.NewS3ObjectLockStore(
			s3Client,
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
		// EPIC #511 Gap 1: the S3 Object Lock legal-hold lift on a
		// released tenant's already-archived objects is NOT triggered
		// from the SetLegalHold RPC. The OFF path durably enqueues a
		// legal_hold_lift_queue row (apply -> globalstore, same txn that
		// clears the hold); the crash-durable LiftWorker wired below
		// drains that queue. archiveStore is kept for that worker.
		log.Printf("entdb-server: S3 Object Lock archive enabled (bucket=%s group=%s)", strings.TrimSpace(*archiveBucket), *archiveGroup)
	}

	applier, err := apply.New(apply.Options{
		Store:               canonical,
		Global:              global,
		Consumer:            walImpl,
		Topic:               *walTopic,
		GroupID:             *walGroup,
		MaxApplyConcurrency: *applyConcurrency,
	})
	if err != nil {
		log.Fatalf("entdb-server: applier: %v", err)
	}
	effectiveApplyConc := *applyConcurrency
	if effectiveApplyConc <= 0 {
		effectiveApplyConc = runtime.GOMAXPROCS(0)
	}
	if effectiveApplyConc == 1 {
		log.Printf("entdb-server: WAL apply concurrency = 1 (strictly serial; pre-#140 behaviour)")
	} else {
		log.Printf("entdb-server: WAL apply concurrency = %d (cross-tenant parallel apply, ADR-027)", effectiveApplyConc)
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
	gdprErr := make(chan error, 1)
	if *gdprWorkerEnabled {
		gdprProcessor, err := gdpr.New(gdpr.Options{
			Global:                global,
			Store:                 canonical,
			Keys:                  keyManager,
			RemoveCiphertextFiles: *cryptoShredDeleteFiles,
		})
		if err != nil {
			log.Fatalf("entdb-server: gdpr worker: %v", err)
		}
		go func() {
			gdprErr <- gdprProcessor.Run(ctx, *gdprWorkerInterval)
		}()
		log.Printf("entdb-server: GDPR deletion worker enabled (interval=%s crypto-shred-delete-files=%v)", *gdprWorkerInterval, *cryptoShredDeleteFiles)
	}

	// EPIC #511 Gap 1: the durable legal-hold-lift worker. SetLegalHold
	// OFF durably enqueues a legal_hold_lift_queue row (in the same
	// globalstore txn that clears the hold); this worker drains the
	// queue, running the idempotent/resumable paginated S3 sweep for
	// each released tenant and retrying on the next tick until complete.
	// It replaces the previous detached fire-and-forget goroutine, which
	// was not crash-durable. Only does work when the archive sidecar is
	// enabled (archiveStore != nil); otherwise there is no S3 to sweep
	// and the queue stays empty (the OFF path is a pure status flip).
	liftErr := make(chan error, 1)
	if *legalHoldLiftWorkerEnabled && archiveStore != nil {
		liftWorker, err := audit.NewLiftWorker(audit.LiftWorkerOptions{
			Queue:  legalHoldLiftQueueAdapter{global: global},
			Lifter: archiveStore,
		})
		if err != nil {
			log.Fatalf("entdb-server: legal-hold lift worker: %v", err)
		}
		go func() {
			liftErr <- liftWorker.Run(ctx, *legalHoldLiftWorkerInterval)
		}()
		log.Printf("entdb-server: legal-hold-lift worker enabled (interval=%s)", *legalHoldLiftWorkerInterval)
	} else if *legalHoldLiftWorkerEnabled {
		log.Printf("entdb-server: legal-hold-lift worker idle (archive sidecar disabled; nothing to sweep)")
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
			// Seed through the shared producer; the applier goroutine
			// started above consumes the event and writes the receipt +
			// offset rows (GitHub issue #505).
			if err := testseed.SeedTenantContract(ctx, global, canonical, walImpl, *walTopic, *seedTenant); err != nil {
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

	grpcServerOpts, tlsEnabled, tlsReloader, err := grpcServerTLSOptions(serverTLSConfig{
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
		grpcServerOpts = append(grpcServerOpts,
			grpc.ChainUnaryInterceptor(auth.MTLSPeerIdentityUnaryInterceptor()),
			grpc.ChainStreamInterceptor(auth.MTLSPeerIdentityStreamInterceptor()),
		)
		log.Printf("entdb-server: TLS enabled (min-version=%s client-auth=%s)", *tlsMinVersion, clientAuthMode(*tlsCA, *requireClientCert))
	} else {
		log.Printf("entdb-server: WARNING: TLS disabled; plaintext gRPC listener is for local/dev use only")
	}

	// Production OAuth/OIDC. When any -oauth-* flag is set we build a
	// network-backed JWKSValidator (real JWKS fetch + caching + key
	// rotation, OIDC discovery, provider presets) and install the auth
	// interceptor. The interceptor reads gRPC metadata, so it is wired
	// independently of TLS. The session manager stays nil here
	// (in-memory dev manager and its flag are tracked under sibling
	// issue #88); a nil backend simply means that credential type is
	// not accepted.
	oc := oauthConfig{
		provider: *oauthProvider,
		issuer:   *oauthIssuer,
		jwksURL:  *oauthJWKSURL,
		audience: *oauthAudience,
	}
	if oc.enabled() {
		validator, err := buildOAuthValidator(ctx, oc)
		if err != nil {
			log.Fatalf("entdb-server: OAuth config: %v", err)
		}
		authInterceptor := auth.NewInterceptor(validator, nil, nil)
		grpcServerOpts = append(grpcServerOpts,
			grpc.ChainUnaryInterceptor(authInterceptor.Unary()),
			grpc.ChainStreamInterceptor(authInterceptor.Stream()),
		)
		log.Printf("entdb-server: OAuth/OIDC authentication enabled (%s)", oauthSummary(oc, validator))
	} else {
		log.Printf("entdb-server: WARNING: OAuth/OIDC authentication disabled; no -oauth-* flags set (local/dev only — production MUST set --oauth-issuer)")
	}

	// Persistent API-key auth. When enabled, every non-Health RPC must
	// present a valid x-api-key whose argon2id hash is stored in
	// global.db. Rotation is supported out of band (issue a new key,
	// flip clients, revoke the old key_id) -- both keys validate while
	// both are active. Installed AFTER the mTLS peer interceptor so a
	// future authorization layer sees the API-key Identity on the
	// context.
	if *apiKeyAuth {
		keyMgr := auth.NewPersistentAPIKeyManager(globalStoreAPIKeys{global})
		apiKeyInterceptor := auth.NewInterceptor(nil, keyMgr, nil)
		grpcServerOpts = append(grpcServerOpts,
			grpc.ChainUnaryInterceptor(apiKeyInterceptor.Unary()),
			grpc.ChainStreamInterceptor(apiKeyInterceptor.Stream()),
		)
		log.Printf("entdb-server: persistent API-key auth enabled (argon2id, global.db-backed, rotatable)")
	}

	srv := grpc.NewServer(grpcServerOpts...)
	pb.RegisterEntDBServiceServer(srv, api.New(srvOpts...))

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("entdb-server: listen %s: %v", *addr, err)
	}

	// Prometheus scrape endpoint. Off by default; production / e2e set
	// --metrics-addr so dashboards and alert rules (entdb_archive_lag_events,
	// entdb_grpc_*) have a target. Served on a plain HTTP listener
	// separate from the gRPC port — scrape traffic is in-cluster.
	var metricsSrv *http.Server
	if strings.TrimSpace(*metricsAddr) != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		metricsSrv = &http.Server{
			Addr:              strings.TrimSpace(*metricsAddr),
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		metricsLis, err := net.Listen("tcp", metricsSrv.Addr)
		if err != nil {
			log.Fatalf("entdb-server: metrics listen %s: %v", metricsSrv.Addr, err)
		}
		go func() {
			if err := metricsSrv.Serve(metricsLis); err != nil && err != http.ErrServerClosed {
				log.Printf("entdb-server: metrics endpoint exited: %v", err)
			}
		}()
		log.Printf("entdb-server: Prometheus metrics on %s/metrics", metricsSrv.Addr)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	if tlsReloader != nil {
		reload := make(chan os.Signal, 1)
		signal.Notify(reload, syscall.SIGHUP)
		go func() {
			defer signal.Stop(reload)
			for {
				select {
				case <-ctx.Done():
					return
				case <-reload:
					if err := tlsReloader.Reload(); err != nil {
						log.Printf("entdb-server: TLS reload failed: %v", err)
					} else {
						log.Printf("entdb-server: TLS cert/key/CA reloaded after SIGHUP")
					}
				}
			}
		}()
	}
	go func() {
		<-stop
		log.Printf("entdb-server: shutting down")
		applier.Stop()
		cancel()
		if metricsSrv != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = metricsSrv.Shutdown(shutdownCtx)
			shutdownCancel()
		}
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
	case err := <-gdprErr:
		if err != nil && err != context.Canceled {
			log.Printf("entdb-server: gdpr worker exited: %v", err)
		}
	case err := <-liftErr:
		if err != nil && err != context.Canceled {
			log.Printf("entdb-server: legal-hold-lift worker exited: %v", err)
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

// globalStoreAPIKeys adapts *globalstore.GlobalStore to
// auth.APIKeyStore. The adapter lives here -- the only place that wires
// both packages -- so neither auth nor globalstore takes a dependency on
// the other (the cross-package edge is owned by main, like every other
// backend wiring in this file).
type globalStoreAPIKeys struct {
	g *globalstore.GlobalStore
}

func (a globalStoreAPIKeys) PutAPIKey(ctx context.Context, k auth.StoredAPIKey) error {
	return a.g.PutAPIKey(ctx, globalstore.APIKeyRecord{
		KeyID:     k.KeyID,
		Name:      k.Name,
		Hash:      k.Hash,
		Scopes:    k.Scopes,
		Status:    globalstore.APIKeyStatusActive,
		ExpiresAt: k.ExpiresAt,
	})
}

func (a globalStoreAPIKeys) RevokeAPIKey(ctx context.Context, keyID string) (bool, error) {
	return a.g.RevokeAPIKey(ctx, keyID)
}

func (a globalStoreAPIKeys) ListActiveAPIKeys(ctx context.Context, now int64) ([]auth.StoredAPIKey, error) {
	recs, err := a.g.ListActiveAPIKeys(ctx, now)
	if err != nil {
		return nil, err
	}
	out := make([]auth.StoredAPIKey, 0, len(recs))
	for _, r := range recs {
		out = append(out, auth.StoredAPIKey{
			KeyID:     r.KeyID,
			Name:      r.Name,
			Hash:      r.Hash,
			Scopes:    r.Scopes,
			ExpiresAt: r.ExpiresAt,
		})
	}
	return out, nil
}

// firstNonEmpty returns the first argument whose trimmed form is
// non-empty, or "" if all are blank. Used to default a cloud backend's
// stream/topic/subscription to the shared --wal-topic / --wal-group
// when its dedicated flag is unset.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func effectiveKMSProvider(provider string) string {
	if strings.TrimSpace(provider) == "" {
		return "file"
	}
	return strings.TrimSpace(provider)
}

// legalHoldLiftQueueAdapter adapts *globalstore.GlobalStore to the
// audit.LiftQueueStore the durable lift worker drains (EPIC #511 Gap 1).
// It is a thin shim: the only real work is mapping the globalstore
// queue-entry slice to the audit package's local type so internal/audit
// does not import internal/globalstore.
type legalHoldLiftQueueAdapter struct {
	global *globalstore.GlobalStore
}

func (a legalHoldLiftQueueAdapter) GetLegalHoldLiftQueue(ctx context.Context) ([]audit.LiftQueueEntry, error) {
	rows, err := a.global.GetLegalHoldLiftQueue(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]audit.LiftQueueEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, audit.LiftQueueEntry{TenantID: r.TenantID, EnqueuedAt: r.EnqueuedAt})
	}
	return out, nil
}

func (a legalHoldLiftQueueAdapter) DequeueLegalHoldLift(ctx context.Context, tenantID string) (bool, error) {
	return a.global.DequeueLegalHoldLift(ctx, tenantID)
}

func (a legalHoldLiftQueueAdapter) CountLegalHoldLiftQueue(ctx context.Context) (int, error) {
	return a.global.CountLegalHoldLiftQueue(ctx)
}

func (a legalHoldLiftQueueAdapter) IsLegalHoldSet(ctx context.Context, tenantID string) (bool, error) {
	return a.global.IsLegalHoldSet(ctx, tenantID)
}
