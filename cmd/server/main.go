package main

import (
	"context"
	"goont/config"
	"goont/handlers"
	"goont/middleware"
	"goont/storage"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := config.Load()

	pool, err := storage.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Error al conectar a la base de datos: %v", err)
	}
	defer pool.Close()

	if err := storage.Migrate(context.Background(), pool); err != nil {
		log.Fatalf("Error al inicializar la base de datos: %v", err)
	}

	handlers.SetStore(storage.New(pool))

	mux := http.NewServeMux()
	setupRoutes(mux)

	handler := middleware.Logging(mux)
	handler = middleware.RecoverPanic(handler)
	handler = middleware.CORS(handler)

	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Servidor iniciado en http://%s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error al iniciar servidor: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Apagando servidor...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Error en shutdown graceful: %v", err)
	}

	log.Println("Servidor apagado correctamente")
}

func setupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/olt", handlers.GetAllOLT)
	mux.HandleFunc("GET /api/v1/olt/{ip}", handlers.GetOLT)
	mux.HandleFunc("GET /api/v1/olt/{ip}/ports", handlers.GetOLTPorts)
	mux.HandleFunc("GET /api/v1/olt/{ip}/onts", handlers.GetOLTONTs)

	mux.HandleFunc("GET /api/v1/traffic/{ip}", handlers.GetTrafficGpons)
	mux.HandleFunc("GET /api/v1/traffic/{ip}/{gpon}", handlers.GetTrafficONTS)
	mux.HandleFunc("GET /api/v1/traffic/{ip}/{gpon}/{ont}", handlers.GetTrafficONT)

	mux.HandleFunc("GET /api/v1/health", handlers.HealthCheck)
	mux.HandleFunc("GET /", handlers.Index)
}
