package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"goodfood/menu-service/internal/adapter/authclient"
	httpadapter "goodfood/menu-service/internal/adapter/http"
	"goodfood/menu-service/internal/adapter/postgres"
	"goodfood/menu-service/internal/application"
	"goodfood/menu-service/internal/config"
)

func main() {
	log := newLogger()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := connectWithRetry(ctx, log, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("database unreachable")
	}
	defer pool.Close()

	if err := postgres.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatal().Err(err).Msg("migrations failed")
	}
	log.Info().Msg("migrations applied")

	uc := application.NewUseCases(postgres.NewMenuRepository(pool))

	// Seed a starter menu per restaurant (best-effort, non-blocking startup).
	go seedMenus(log, cfg.FranchiseServiceURL, uc)

	router := httpadapter.NewRouter(
		httpadapter.NewMenuHandler(uc),
		cfg.JWTSecret,
		log,
		func(ctx context.Context) error { return pool.Ping(ctx) },
	)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router, ReadHeaderTimeout: 5 * time.Second}
	log.Info().Str("port", cfg.Port).Msg("menu-service started")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server stopped")
	}
}

// seedMenus fetches restaurants from auth-service and seeds a starter menu for
// those without one. Retries a few times while auth-service boots.
func seedMenus(log zerolog.Logger, authURL string, uc *application.UseCases) {
	client := authclient.New(authURL)
	for attempt := 1; attempt <= 10; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		tenants, err := client.ListTenants(ctx)
		if err != nil {
			cancel()
			log.Warn().Err(err).Int("attempt", attempt).Msg("cannot reach auth-service for seeding, retrying")
			time.Sleep(3 * time.Second)
			continue
		}
		ids := make([]string, 0, len(tenants))
		for _, t := range tenants {
			if t.IsActive {
				ids = append(ids, t.ID)
			}
		}
		created, err := uc.SeedRestaurantMenus(ctx, ids)
		cancel()
		if err != nil {
			log.Warn().Err(err).Msg("menu seeding failed")
			return
		}
		log.Info().Int("restaurants", len(ids)).Int("items_created", created).Msg("menu seeding done")
		return
	}
	log.Warn().Msg("gave up seeding menus after retries")
}

func newLogger() zerolog.Logger {
	level, err := zerolog.ParseLevel(strings.ToLower(os.Getenv("LOG_LEVEL")))
	if err != nil || level == zerolog.NoLevel {
		level = zerolog.InfoLevel
	}
	return zerolog.New(os.Stdout).Level(level).With().Timestamp().Str("service", "menu-service").Logger()
}

func connectWithRetry(ctx context.Context, log zerolog.Logger, url string) (*pgxpool.Pool, error) {
	for i := 1; ; i++ {
		pool, err := postgres.Connect(ctx, url)
		if err == nil {
			return pool, nil
		}
		if i >= 15 {
			return nil, err
		}
		log.Warn().Err(err).Int("attempt", i).Msg("database not ready, retrying in 2s")
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
