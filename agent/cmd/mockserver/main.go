package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/Payton-Naomi/LogMaster/agent/internal/mockserver"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8081", "listen address")
	dir := flag.String("data-dir", "data/mock-received", "directory for accepted batches")
	failFirst := flag.Int64("fail-first", 0, "return HTTP 500 for the first N requests")
	flag.Parse()
	server := &mockserver.Server{Dir: *dir, FailFirst: *failFirst}
	log.Printf("mock receiver listening on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, server.Handler()))
}
