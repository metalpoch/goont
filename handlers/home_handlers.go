package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HealthCheck - Verificar estado del servidor
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]any{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HomePage - Página de inicio
func HomePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	fmt.Fprintf(w, "Bienvenido a mi API REST")
}
