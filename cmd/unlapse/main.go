// Command unlapse serves the unlapse web app: a local-first dashboard for
// everything that expires — subscriptions, warranties, insurance, documents.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/DynamycSound/unlapse/internal/server"
	"github.com/DynamycSound/unlapse/internal/store"
)

func main() {
	defaultData := os.Getenv("UNLAPSE_DATA")
	if defaultData == "" {
		defaultData = "unlapse-data.json"
	}
	defaultAddr := os.Getenv("UNLAPSE_ADDR")
	if defaultAddr == "" {
		defaultAddr = "127.0.0.1:8275"
	}

	addr := flag.String("addr", defaultAddr, "address to listen on (use 0.0.0.0:8275 to expose on your network)")
	dataPath := flag.String("data", defaultData, "path to the JSON data file")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Println("unlapse", server.Version)
		return
	}

	st, err := store.Open(*dataPath)
	if err != nil {
		log.Fatalf("unlapse: %v", err)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           server.New(st),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("unlapse %s — your data stays in %s", server.Version, *dataPath)
	log.Printf("open http://%s in your browser", displayAddr(*addr))
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("unlapse: %v", err)
	}
}

func displayAddr(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
		return "localhost" + addr
	}
	if len(addr) >= 8 && addr[:8] == "0.0.0.0:" {
		return "localhost:" + addr[8:]
	}
	return addr
}
