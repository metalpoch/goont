package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthCheck - Verificar estado del servidor
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]any{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"version":   "2.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Index - Lista de endpoints disponibles
func Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	response := map[string]any{
		"service": "goont",
		"version": "2.0.0",
		"endpoints": []string{
			"GET /api/v1/health",
			"GET /api/v1/olt",
			"GET /api/v1/olt/{ip}",
			"GET /api/v1/traffic/{ip}",
			"GET /api/v1/traffic/{ip}/{gpon}",
			"GET /api/v1/traffic/{ip}/{gpon}/{ont}",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
