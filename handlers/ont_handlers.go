package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// GetTrafficGpons - Trafico total del OLT (suma de todos sus puertos GPON)
func GetTrafficGpons(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	if ip == "" {
		http.Error(w, "IP parameter required", http.StatusBadRequest)
		return
	}

	dates, err := parseDate(r.URL.Query().Get("initDate"), r.URL.Query().Get("endDate"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	measurements, err := store.OltTraffic(r.Context(), ip, dates.InitDate, dates.EndDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(measurements)
}

// GetTrafficONTS - Trafico de un puerto GPON (contadores del puerto) y estado de sus ONT
func GetTrafficONTS(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	gponStr := r.PathValue("gpon")
	if ip == "" || gponStr == "" {
		http.Error(w, "IP and GPON parameters required", http.StatusBadRequest)
		return
	}

	gponIdx, err := parseGponIdx(gponStr)
	if err != nil {
		http.Error(w, "GPON must be an integer", http.StatusBadRequest)
		return
	}

	dates, err := parseDate(r.URL.Query().Get("initDate"), r.URL.Query().Get("endDate"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	measurements, err := store.GponTrafficData(r.Context(), ip, gponIdx, dates.InitDate, dates.EndDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(measurements)
}

// GetTrafficONT - Trafico de un ONT especifico
func GetTrafficONT(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	gponStr := r.PathValue("gpon")
	ontStr := r.PathValue("ont")
	if ip == "" || gponStr == "" || ontStr == "" {
		http.Error(w, "IP, GPON and ONT parameters required", http.StatusBadRequest)
		return
	}

	gponIdx, err := parseGponIdx(gponStr)
	if err != nil {
		http.Error(w, "GPON must be an integer", http.StatusBadRequest)
		return
	}
	ontIdx, err := strconv.Atoi(ontStr)
	if err != nil {
		http.Error(w, "ONT must be an integer", http.StatusBadRequest)
		return
	}

	dates, err := parseDate(r.URL.Query().Get("initDate"), r.URL.Query().Get("endDate"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	measurements, err := store.OntTraffic(r.Context(), ip, gponIdx, ontIdx, dates.InitDate, dates.EndDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(measurements)
}
