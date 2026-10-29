package http

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

type HealthChecker func(ctx context.Context) error

// publicCacheSeconds is how long the public catalogue may be cached: menus
// change rarely, and a dish made unavailable is refused at checkout anyway.
const publicCacheSeconds = 30

func NewRouter(handler *MenuHandler, jwtCfg JWTConfig, serviceKey string, log zerolog.Logger, dbCheck HealthChecker) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(RequestID)
	r.Use(Logger(log))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if err := dbCheck(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	r.Get("/docs/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(openAPISpec)
	})
	r.Get("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(scalarHTML)
	})

	r.Route("/api/menu", func(r chi.Router) {
		// Public catalog for customers (browse a restaurant's dishes)
		r.With(PublicCache(publicCacheSeconds)).Get("/", handler.ListPublic)
		r.With(PublicCache(publicCacheSeconds)).Get("/by-ids", handler.ListByIDs)

		// Manager (own restaurant) / admin (global catalog) management
		r.Group(func(r chi.Router) {
			r.Use(Auth(jwtCfg))
			r.Get("/manage", handler.ListMine)
			r.Post("/", handler.Create)
			r.Patch("/{id}", handler.Update)
			r.Patch("/{id}/toggle", handler.ToggleAvailability)
			r.Delete("/{id}", handler.Delete)
			r.Get("/{id}/recipe", handler.GetRecipe)
			r.Put("/{id}/recipe", handler.SetRecipe)
		})
	})

	r.Route("/api/menu-categories", func(r chi.Router) {
		// Public — every dish form (franchisee or admin) needs the list
		r.With(PublicCache(publicCacheSeconds)).Get("/", handler.ListCategories)

		// Head office only
		r.Group(func(r chi.Router) {
			r.Use(Auth(jwtCfg))
			r.Post("/", handler.CreateCategory)
			r.Delete("/{id}", handler.DeleteCategory)
		})
	})

	r.Route("/api/menu-plans", func(r chi.Router) {
		// Public catalog for customers (browse a restaurant's menus/formules)
		r.With(PublicCache(publicCacheSeconds)).Get("/", handler.ListPlansPublic)

		// Manager (own restaurant) / admin (global catalog) management
		r.Group(func(r chi.Router) {
			r.Use(Auth(jwtCfg))
			r.Get("/manage", handler.ListPlansMine)
			r.Post("/", handler.CreatePlan)
			r.Patch("/{id}", handler.UpdatePlan)
			r.Patch("/{id}/toggle", handler.TogglePlanAvailability)
			r.Delete("/{id}", handler.DeletePlan)
		})
	})

	// Service-to-service routes: deliberately NOT under /api, so the ingress
	// (which only routes /api/...) never exposes them outside the cluster.
	r.Route("/internal", func(r chi.Router) {
		r.Use(ServiceKey(serviceKey))
		r.Post("/recipes/lookup", handler.LookupRecipes)
		r.Post("/recipes/usage", handler.StockItemUsage)
	})

	return r
}
