// Command entdb-server is the Go reimplementation of the EntDB
// gRPC server (tracking issue #407). It is intentionally minimal
// in Phase 0: it parses flags, binds a gRPC server, and registers
// an EntDBService that returns codes.Unimplemented for every RPC.
// Real handlers land one PR at a time as RPC sub-issues are
// completed.
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

func main() {
	addr := flag.String("addr", ":50051", "gRPC bind address (host:port)")
	flag.Parse()

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("entdb-server: listen %s: %v", *addr, err)
	}

	srv := grpc.NewServer()
	pb.RegisterEntDBServiceServer(srv, api.New())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		log.Printf("entdb-server: shutting down")
		srv.GracefulStop()
	}()

	log.Printf("entdb-server: listening on %s (all RPCs return Unimplemented)", *addr)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("entdb-server: serve: %v", err)
	}
}
