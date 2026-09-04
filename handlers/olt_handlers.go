package handlers

import (
	"encoding/json"
	"goont/storage"
	"net/http"
)

var store *storage.Store

func SetStore(s *storage.Store) {
	store = s
}

// GetAllOLT - Obtener todos los olt en medicion
func GetAllOLT(w http.ResponseWriter, r *http.Request) {
	olts, err := store.GetInfoOLTs(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(olts)
}

// GetOLT - Obtener un olt por IP
func GetOLT(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	if ip == "" {
		http.Error(w, "IP parameter required", http.StatusBadRequest)
		return
	}

	olt, err := store.GetOLTByID(r.Context(), ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if olt == nil {
		http.Error(w, "OLT not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(olt)
}
