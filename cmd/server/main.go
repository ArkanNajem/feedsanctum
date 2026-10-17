package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ArkanNajem/feedsanctum/internal/database"
	"github.com/ArkanNajem/feedsanctum/internal/docs"
	"github.com/ArkanNajem/feedsanctum/internal/handlers"
	"github.com/ArkanNajem/feedsanctum/internal/scraper"
	swgui "github.com/swaggest/swgui/v5emb"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {

	godotenv.Load()

	port := os.Getenv("PORT")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
			os.Getenv("DB_HOST"),
			os.Getenv("DB_PORT"),
			os.Getenv("DB_NAME"),
		)
	}

	dbConnection, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("err connecting to database: %v", err)
	}

	defer dbConnection.Close()

	dbQueries := database.New(dbConnection)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	apicfg := &handlers.APIConfig{
		DB:           dbQueries,
		DBConn:       dbConnection,
		JWTSecret:    os.Getenv("JWT_SECRET"),
		JWTExpiry:    time.Hour,
		Ticker:       time.Hour,
		LoginLimiter: httprate.NewRateLimiter(8, time.Minute, httprate.WithLimitHandler(rateLimitErrHandler)),
		ServerCTX:    ctx,
	}

	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)

	r.Use(middleware.ClientIPFromRemoteAddr)
	r.Use(httprate.LimitBy(60, time.Minute, httprate.JoinKeys(clientIPKey, httprate.KeyByEndpoint),
		httprate.WithLimitHandler(rateLimitErrHandler)))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := dbConnection.PingContext(r.Context()); err != nil {
			http.Error(w, "db down", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/register", apicfg.HandlerCreateUser)
		r.With(httprate.LimitBy(5, time.Minute, clientIPKey, httprate.WithLimitHandler(rateLimitErrHandler))).Post("/login", apicfg.HandlerLogin)
		r.Post("/refresh", apicfg.HandlerRefresh)
		r.Post("/logout", apicfg.HandlerLogOut)
		r.Get("/feeds", apicfg.HandlerGetAllFeeds)
		r.Get("/feeds/{feedID}", apicfg.HandlerGetFeedByID)

		r.Group(func(r chi.Router) {
			r.Use(apicfg.MiddlewareAuth)
			r.Get("/me", apicfg.HandlerGetCurrentUser)
			r.Patch("/me", apicfg.HandlerUpdateUser)
			r.Delete("/me", apicfg.HandlerDeleteUser)
			r.Post("/feeds", apicfg.HandlerCreateFeed)
			r.Get("/follows", apicfg.HandlerGetFeedFollows)
			r.Post("/follows", apicfg.HandlerCreateFeedFollow)
			r.Delete("/follows/{feedID}", apicfg.HandlerDeleteFeedFollow)
			r.Get("/posts", apicfg.HandlerGetPostsForUser)
			r.Post("/posts/{postID}/read", apicfg.HandlerMarkPostRead)
			r.Get("/posts/read", apicfg.HandlerGetReadPostsForUser)
		})

	})

	r.Get("/openapi.yaml", docs.HandleSpec)
	r.Mount("/docs", swgui.New("FeedSanctum API", "/openapi.yaml", "/docs"))

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go scraper.StartScraping(apicfg.ServerCTX, apicfg.DB, apicfg.Ticker)
	go func() {
		fmt.Printf("serving on %s\n", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()
	<-ctx.Done()
	log.Println("shutdown signal received, exiting...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server forced to shutdown, %v", err)
	}
}

func clientIPKey(r *http.Request) (string, error) {
	return httprate.CanonicalizeIP(middleware.GetClientIP(r.Context())), nil
}

func rateLimitErrHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"error": "Rate limit reached"}`))
}
