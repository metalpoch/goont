package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// GetOLTPorts - Enumerar los puertos GPON de un OLT con conteo de ONTs
func GetOLTPorts(w http.ResponseWriter, r *http.Request) {
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

	ports, err := store.ListOltPorts(r.Context(), ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ports)
}

// GetOLTONTs - Inventario de ONTs de un OLT, buscable por descripcion (DNI/RIF) o serial
func GetOLTONTs(w http.ResponseWriter, r *http.Request) {
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

	query := r.URL.Query().Get("q")

	var gponIdx *int
	if g := r.URL.Query().Get("gpon"); g != "" {
		idx, err := strconv.Atoi(g)
		if err != nil {
			http.Error(w, "GPON must be an integer", http.StatusBadRequest)
			return
		}
		gponIdx = &idx
	}

	onts, err := store.ListOnts(r.Context(), ip, gponIdx, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(onts)
}
