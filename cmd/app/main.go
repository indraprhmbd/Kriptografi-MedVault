// Command app runs the MedVault dashboard and REST API.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"medvault/internal/biometric"
	"medvault/internal/config"
	"medvault/internal/crypto"
	"medvault/internal/database"
	"medvault/internal/handler"
	"medvault/internal/ledger"
	"medvault/internal/repository"
	"medvault/web"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	column, err := crypto.NewColumnEncryptor(cfg.MasterAES256Key)
	if err != nil {
		log.Fatalf("column encryptor: %v", err)
	}
	super, err := crypto.NewSuperCipher(cfg.VigenereKey, cfg.TranspositionKey, cfg.MasterRootSecret)
	if err != nil {
		log.Fatalf("super cipher: %v", err)
	}
	classical, err := crypto.NewClassicalCipher(cfg.VigenereKey, cfg.TranspositionKey)
	if err != nil {
		log.Fatalf("classical cipher: %v", err)
	}
	vault, err := crypto.NewLabVault(cfg.MasterRootSecret)
	if err != nil {
		log.Fatalf("lab vault: %v", err)
	}

	store, err := repository.NewStore(db, column)
	if err != nil {
		log.Fatalf("repository: %v", err)
	}

	webauthnService, err := biometric.NewWebAuthnService(biometric.WebAuthnConfig{
		RPDisplayName: cfg.WebAuthnRPDisplayName,
		RPID:          cfg.WebAuthnRPID,
		RPOrigins:     cfg.WebAuthnRPOrigins,
	})
	if err != nil {
		log.Fatalf("webauthn: %v", err)
	}

	validatorClient, err := ledger.NewClient(cfg.ValidatorURL, ledger.DefaultClientTimeout)
	if err != nil {
		log.Fatalf("validator client: %v", err)
	}
	if !cfg.ValidatorEnabled() {
		log.Printf("warning: validator key pair is not configured, the gateway must still run for blocks to seal")
	}

	webHandler, err := web.Handler(cfg.DevWebDir)
	if err != nil {
		log.Fatalf("web assets: %v", err)
	}

	api, err := handler.New(handler.Options{
		Config:    cfg,
		Store:     store,
		Super:     super,
		Classical: classical,
		Vault:     vault,
		Column:    column,
		StegoKey:  cfg.StegoHMACKey,
		Validator: validatorClient,
		WebAuthn:  webauthnService,
		Web:       webHandler,
	})
	if err != nil {
		log.Fatalf("handler: %v", err)
	}

	server := &http.Server{
		Addr:              cfg.AppAddr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	maintenanceCtx, stopMaintenance := context.WithCancel(context.Background())
	defer stopMaintenance()
	go runMaintenance(maintenanceCtx, store)

	go func() {
		for key, value := range cfg.Redacted() {
			log.Printf("config %s=%s", key, value)
		}
		log.Printf("medvault app listening on %s", cfg.AppAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	stopMaintenance()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Print("medvault app stopped")
}

// runMaintenance expires stale challenges and sessions so the SQLite file does
// not grow unbounded on a long running demo.
func runMaintenance(ctx context.Context, store *repository.Store) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			challenges, err := store.Challenges.CleanupExpired(cleanupCtx)
			if err != nil {
				log.Printf("cleanup challenges: %v", err)
			}
			sessions, err := store.AppSessions.CleanupExpired(cleanupCtx)
			if err != nil {
				log.Printf("cleanup sessions: %v", err)
			}
			ceremonies, err := store.WebSessions.CleanupExpired(cleanupCtx)
			if err != nil {
				log.Printf("cleanup ceremonies: %v", err)
			}
			cancel()
			if challenges+sessions+ceremonies > 0 {
				log.Printf("maintenance removed challenges=%d sessions=%d ceremonies=%d",
					challenges, sessions, ceremonies)
			}
		}
	}
}
