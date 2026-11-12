// Command validator runs the Proof-of-Authority gateway. It listens on the
// loopback interface only and is never exposed through the Cloudflare tunnel.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"medvault/internal/ledger"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	addr := envOr("MEDVAULT_VALIDATOR_ADDR", "127.0.0.1:8096")
	privHex := strings.TrimSpace(os.Getenv("MEDVAULT_VALIDATOR_PRIVKEY_HEX"))
	if privHex == "" {
		log.Fatal("MEDVAULT_VALIDATOR_PRIVKEY_HEX is required")
	}

	signer, err := ledger.NewSignerFromSeed(privHex)
	if err != nil {
		log.Fatalf("load validator key: %v", err)
	}

	if expected := strings.TrimSpace(os.Getenv("MEDVAULT_VALIDATOR_PUBKEY_HEX")); expected != "" &&
		!strings.EqualFold(expected, signer.PublicKeyHex()) {
		log.Fatal("MEDVAULT_VALIDATOR_PUBKEY_HEX does not match the configured private key")
	}

	gateway, err := ledger.NewGateway(signer)
	if err != nil {
		log.Fatalf("build gateway: %v", err)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           gateway.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("validator gateway listening on %s", addr)
		log.Printf("validator public key %s", signer.PublicKeyHex())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("validator gateway: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("validator shutdown: %v", err)
	}
	log.Print("validator gateway stopped")
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
