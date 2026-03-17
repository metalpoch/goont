package handlers

import (
	"encoding/json"
	"fmt"
	"goont/storage"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func getOntDB(ip string) (*storage.OntClient, error) {
	dbPath := filepath.Join(dataDir, ip)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("ONT database for IP %s not found", ip)
	}
	client, err := storage.NewOntDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open ONT database for IP %s: %w", ip, err)
	}
	return client, nil
}

// parseTimeParam parses a time parameter from query string.
// Supports RFC3339 format (e.g., "2006-01-02T15:04:05Z07:00").
func parseTimeParam(param string) (time.Time, error) {
	if param == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, param)
}

func GetTrafficOLT(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	if ip == "" {
		http.Error(w, "IP parameter required", http.StatusBadRequest)
		return
	}

	// Parse time parameters
	initDateStr := r.URL.Query().Get("initDate")
	endDateStr := r.URL.Query().Get("endDate")

	if initDateStr == "" || endDateStr == "" {
		http.Error(w, "Both initDate and endDate must be provided when using date range", http.StatusBadRequest)
		return
	}

	initTime, err := parseTimeParam(initDateStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid initDate format: %v", err), http.StatusBadRequest)
		return
	}

	endTime, err := parseTimeParam(endDateStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid endDate format: %v", err), http.StatusBadRequest)
		return
	}

	if endTime.Before(initTime) {
		http.Error(w, "endDate must be after initDate", http.StatusBadRequest)
		return
	}

	client, err := getOntDB(ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	measurements, err := client.GetMeasurements(initTime, endTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(measurements)
}

func GetTrafficGpon(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	gponStr := r.PathValue("gpon")
	if ip == "" || gponStr == "" {
		http.Error(w, "IP and GPON parameters required", http.StatusBadRequest)
		return
	}

	gponIdx, err := strconv.Atoi(gponStr)
	if err != nil {
		http.Error(w, "GPON must be an integer", http.StatusBadRequest)
		return
	}

	initDateStr := r.URL.Query().Get("initDate")
	endDateStr := r.URL.Query().Get("endDate")

	if initDateStr == "" || endDateStr == "" {
		http.Error(w, "Both initDate and endDate must be provided when using date range", http.StatusBadRequest)
		return
	}

	initTime, err := parseTimeParam(initDateStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid initDate format: %v", err), http.StatusBadRequest)
		return
	}
	endTime, err := parseTimeParam(endDateStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid endDate format: %v", err), http.StatusBadRequest)
		return
	}

	if endTime.Before(initTime) {
		http.Error(w, "endDate must be after initDate", http.StatusBadRequest)
		return
	}

	client, err := getOntDB(ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	measurements, err := client.GetMeasurementsByGpon(gponIdx, initTime, endTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(measurements)
}

func GetTrafficONT(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	gponStr := r.PathValue("gpon")
	ontStr := r.PathValue("ont")
	if ip == "" || gponStr == "" || ontStr == "" {
		http.Error(w, "IP, GPON and ONT parameters required", http.StatusBadRequest)
		return
	}

	gponIdx, err := strconv.Atoi(gponStr)
	if err != nil {
		http.Error(w, "GPON must be an integer", http.StatusBadRequest)
		return
	}
	ontIdx, err := strconv.Atoi(ontStr)
	if err != nil {
		http.Error(w, "ONT must be an integer", http.StatusBadRequest)
		return
	}

	// Parse time parameters
	initDateStr := r.URL.Query().Get("initDate")
	endDateStr := r.URL.Query().Get("endDate")

	if initDateStr == "" || endDateStr == "" {
		http.Error(w, "Both initDate and endDate must be provided when using date range", http.StatusBadRequest)
		return
	}

	initTime, err := parseTimeParam(initDateStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid initDate format: %v", err), http.StatusBadRequest)
		return
	}
	endTime, err := parseTimeParam(endDateStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid endDate format: %v", err), http.StatusBadRequest)
		return
	}

	if endTime.Before(initTime) {
		http.Error(w, "endDate must be after initDate", http.StatusBadRequest)
		return
	}

	client, err := getOntDB(ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	measurements, err := client.GetMeasurementsByOnt(gponIdx, ontIdx, initTime, endTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(measurements)
}
