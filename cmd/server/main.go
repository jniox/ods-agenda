package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/orbus-digital/agenda/internal/api"
	"github.com/orbus-digital/agenda/internal/auth"
	"github.com/orbus-digital/agenda/internal/events"
	"github.com/orbus-digital/agenda/internal/repository"
)

func main() {
	// Logging
	logLevel := os.Getenv("LOG_LEVEL")
	switch strings.ToLower(logLevel) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Str("service", "agenda").Logger()

	// Config
	port := os.Getenv("PORT")
	if port == "" {
		port = "8088"
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal().Msg("DATABASE_URL is required")
	}
	oidIssuerURL := os.Getenv("OID_ISSUER_URL")
	if oidIssuerURL == "" {
		log.Fatal().Msg("OID_ISSUER_URL environment variable is required")
	}
	brokers := os.Getenv("REDPANDA_BROKERS")

	// Database
	ctx := context.Background()
	db, err := repository.NewDB(ctx, dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()
	log.Info().Msg("connected to database")

	// Event producer: Kafka when configured, NoopProducer otherwise.
	// The NoopProducer fallback unblocks Cloud Run deploys (Redpanda lives
	// on the Coolify Docker network, unreachable from the VPC connector).
	// Events are recorded in-memory but not published — acceptable for
	// staging until Pub/Sub adoption.
	var producer events.Producer
	if brokers == "" {
		log.Warn().Msg("REDPANDA_BROKERS unset — using NoopProducer (events dropped)")
		producer = &events.NoopProducer{}
	} else {
		kp := events.NewKafkaProducer(strings.Split(brokers, ","))
		defer kp.Close()
		producer = kp
	}

	// JWT middleware: RS256 via JWKS
	jwksProvider := auth.NewJWKSProvider(oidIssuerURL, 5*time.Minute)
	jwtMw := auth.NewJWTMiddlewareJWKS(jwksProvider, oidIssuerURL)

	// Router
	router := api.NewRouterWithConfig(api.RouterConfig{
		DB:              db,
		Producer:        producer,
		JWTMiddleware:   jwtMw,
		CORSOrigins:     api.CORSOriginsFromEnv(),
		RateLimitPerSec: 10,
		RateLimitBurst:  20,
	})

	// Server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Info().Str("port", port).Msg("agenda service starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	<-done
	log.Info().Msg("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}

	fmt.Println("agenda service stopped")
}
