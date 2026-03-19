package main

import (
	"context"
	"goont/handlers"
	"goont/middleware"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const PORT string = "8080"

func main() {
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("No se pudo obtener ruta del ejecutable: %v", err)
	}
	dataDir := filepath.Dir(exe)
	handlers.SetDataDir(dataDir)

	mux := http.NewServeMux()
	setupRoutes(mux)

	handler := middleware.Logging(mux)
	handler = middleware.RecoverPanic(handler)
	handler = middleware.CORS(handler)

	server := &http.Server{
		// Addr:         ":" + PORT,
		Addr:         "0.0.0.0:8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Servidor iniciado en http://localhost:%s", PORT)
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

	mux.HandleFunc("GET /api/v1/traffic/{ip}", handlers.GetTrafficGpons)
	mux.HandleFunc("GET /api/v1/traffic/{ip}/{gpon}", handlers.GetTrafficONTS)
	mux.HandleFunc("GET /api/v1/traffic/{ip}/{gpon}/{ont}", handlers.GetTrafficONT)

	mux.HandleFunc("GET /api/v1/health", handlers.HealthCheck)
	mux.HandleFunc("GET /", handlers.HomePage)

}
