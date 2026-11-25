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

	cacheCfg := middleware.CacheConfigFromEnv()
	handler := middleware.Timeout(110 * time.Second)(mux)
	handler = middleware.NewCache(cacheCfg)(handler)
	handler = middleware.Logging(handler)
	handler = middleware.RecoverPanic(handler)
	handler = middleware.CORS(handler)

	if cacheCfg.TTL > 0 {
		log.Printf("Caché en memoria activa: TTL %s, máximo %d MiB", cacheCfg.TTL, cacheCfg.MaxBytes>>20)
	}

	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: handler,
		// Las consultas de tráfico agregan días de muestras y tardan decenas
		// de segundos: 15 s cortaba la respuesta. El plazo real lo impone el
		// contexto por petición (middleware.Timeout) y la cancelación del
		// cliente.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
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
	mux.HandleFunc("GET /api/v1/traffic/{ip}/ports", handlers.GetTrafficPorts)
	mux.HandleFunc("GET /api/v1/traffic/{ip}/{gpon}", handlers.GetTrafficONTS)
	mux.HandleFunc("GET /api/v1/traffic/{ip}/{gpon}/{ont}", handlers.GetTrafficONT)

	mux.HandleFunc("GET /api/v1/health", handlers.HealthCheck)
	mux.HandleFunc("GET /", handlers.Index)
}
