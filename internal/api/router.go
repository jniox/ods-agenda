package api

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/orbus-digital/agenda/internal/auth"
	"github.com/orbus-digital/agenda/internal/domain"
	"github.com/orbus-digital/agenda/internal/events"
	"github.com/orbus-digital/agenda/internal/repository"
)

// RouterConfig holds configuration for the router.
type RouterConfig struct {
	DB               *repository.DB
	Producer         events.Producer
	JWTMiddleware    *auth.JWTMiddleware
	CORSOrigins      []string
	RateLimitPerSec  float64
	RateLimitBurst   int
}

// SecurityHeaders adds standard HTTP security headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// RateLimiter implements a simple token-bucket rate limiter per IP.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     float64
	burst    int
}

type visitor struct {
	tokens   float64
	lastSeen time.Time
}

// NewRateLimiter creates a rate limiter with the given rate (tokens/sec) and burst.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		burst:    burst,
	}
	// Background cleanup of stale visitors
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(1 * time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Allow checks if a request from the given IP is allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	now := time.Now()
	if !exists {
		rl.visitors[ip] = &visitor{tokens: float64(rl.burst) - 1, lastSeen: now}
		return true
	}

	elapsed := now.Sub(v.lastSeen).Seconds()
	v.tokens += elapsed * rl.rate
	if v.tokens > float64(rl.burst) {
		v.tokens = float64(rl.burst)
	}
	v.lastSeen = now

	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}

// Handler returns the rate limiting middleware.
func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip = strings.Split(fwd, ",")[0]
		}
		if !rl.Allow(strings.TrimSpace(ip)) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"code":"RATE_LIMITED","message":"too many requests"}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CORSOriginsFromEnv returns CORS origins from the env var, or defaults.
func CORSOriginsFromEnv() []string {
	origins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if origins == "" {
		return []string{"https://*.staging.orbusdigital.com", "https://agenda.staging.orbusdigital.com"}
	}
	return strings.Split(origins, ",")
}

// NewRouter creates the Chi router with all endpoints registered.
// Deprecated: Use NewRouterWithConfig instead.
func NewRouter(db *repository.DB, producer events.Producer, jwtSecret string) chi.Router {
	jwtMw := auth.NewJWTMiddleware(jwtSecret)
	return NewRouterWithConfig(RouterConfig{
		DB:              db,
		Producer:        producer,
		JWTMiddleware:   jwtMw,
		CORSOrigins:     CORSOriginsFromEnv(),
		RateLimitPerSec: 10,
		RateLimitBurst:  20,
	})
}

// NewRouterWithConfig creates the Chi router with full configuration.
func NewRouterWithConfig(cfg RouterConfig) chi.Router {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(SecurityHeaders)
	r.Use(RequestLogger)

	// Rate limiting
	ratePerSec := cfg.RateLimitPerSec
	if ratePerSec <= 0 {
		ratePerSec = 10
	}
	burst := cfg.RateLimitBurst
	if burst <= 0 {
		burst = 20
	}
	limiter := NewRateLimiter(ratePerSec, burst)
	r.Use(limiter.Handler)

	// CORS
	origins := cfg.CORSOrigins
	if len(origins) == 0 {
		origins = CORSOriginsFromEnv()
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Tenant-Id", "X-Correlation-Id", "X-Source-Service"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Repositories
	calendarRepo := repository.NewCalendarRepo(cfg.DB)
	eventRepo := repository.NewEventRepo(cfg.DB)
	attendeeRepo := repository.NewAttendeeRepo(cfg.DB)
	availabilityRepo := repository.NewAvailabilityRepo(cfg.DB)

	// Handlers
	healthHandler := NewHealthHandler(cfg.DB)
	calendarHandler := NewCalendarHandler(calendarRepo, cfg.Producer)
	eventHandler := NewEventHandler(eventRepo, calendarRepo, attendeeRepo, cfg.Producer)
	attendeeHandler := NewAttendeeHandler(attendeeRepo, eventRepo, cfg.Producer)
	availabilityHandler := NewAvailabilityHandler(availabilityRepo)

	// Public endpoints
	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)

	// Protected endpoints
	r.Group(func(r chi.Router) {
		r.Use(cfg.JWTMiddleware.Handler)

		// Calendars
		r.Route("/api/v1/calendars", func(r chi.Router) {
			r.Post("/", calendarHandler.Create)
			r.Get("/", calendarHandler.List)
			r.Get("/{id}", calendarHandler.GetByID)
			r.Put("/{id}", calendarHandler.Update)
			r.Delete("/{id}", calendarHandler.Delete)

			// Events under calendar
			r.Post("/{calendar_id}/events", eventHandler.Create)
			r.Get("/{calendar_id}/events", eventHandler.ListByCalendar)
		})

		// Events (cross-calendar)
		r.Route("/api/v1/events", func(r chi.Router) {
			r.Get("/", eventHandler.ListByTenant)
			r.Get("/{id}", eventHandler.GetByID)
			r.Put("/{id}", eventHandler.Update)
			r.Delete("/{id}", eventHandler.Cancel)

			// Attendees under event
			r.Post("/{event_id}/attendees", attendeeHandler.AddAttendee)
			r.Put("/{event_id}/attendees/{attendee_id}/respond", attendeeHandler.Respond)
			r.Delete("/{event_id}/attendees/{attendee_id}", attendeeHandler.RemoveAttendee)
		})

		// Availability
		r.Post("/api/v1/availability", availabilityHandler.CheckAvailability)
	})

	return r
}

// Verify that repository types implement the domain interfaces at compile time.
var _ domain.CalendarRepository = (*repository.CalendarRepo)(nil)
var _ domain.EventRepository = (*repository.EventRepo)(nil)
var _ domain.AttendeeRepository = (*repository.AttendeeRepo)(nil)
var _ domain.AvailabilityRepository = (*repository.AvailabilityRepo)(nil)
