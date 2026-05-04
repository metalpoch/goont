package handlers

import (
	"encoding/json"
	"fmt"
	"goont/models"
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

func GetTrafficONTS(w http.ResponseWriter, r *http.Request) {
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

	dates, err := parseDate(initDateStr, endDateStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	client, err := getOntDB(ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	measurements, err := client.GetMeasurementsByGpon(gponIdx, dates.InitDate, dates.EndDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := proccessGroupedOnt(measurements)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

	initDateStr := r.URL.Query().Get("initDate")
	endDateStr := r.URL.Query().Get("endDate")

	dates, err := parseDate(initDateStr, endDateStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	client, err := getOntDB(ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	measurements, err := client.GetMeasurementsByOnt(gponIdx, ontIdx, dates.InitDate, dates.EndDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := proccessOnt(measurements)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func GetTrafficGpons(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	if ip == "" {
		http.Error(w, "IP parameter required", http.StatusBadRequest)
		return
	}

	initDateStr := r.URL.Query().Get("initDate")
	endDateStr := r.URL.Query().Get("endDate")

	dates, err := parseDate(initDateStr, endDateStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	client, err := getOntDB(ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	measurements, err := client.GetAllMeasurements(dates.InitDate, dates.EndDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := processGponTraffic(measurements)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func GetOntStatusByDate(w http.ResponseWriter, r *http.Request) {
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

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		http.Error(w, "date parameter required (format: YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	targetDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "Invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	startOfDay := targetDate
	endOfDay := targetDate.Add(24 * time.Hour)

	client, err := getOntDB(ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	measurements, err := client.GetMeasurementsByGpon(gponIdx, startOfDay, endOfDay)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lastMeasurements, err := client.GetLastMeasurementBeforeDate(startOfDay)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	status := calculateOntStatus(measurements, lastMeasurements, gponIdx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func GetOntStatusByOlt(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	if ip == "" {
		http.Error(w, "IP parameter required", http.StatusBadRequest)
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		http.Error(w, "date parameter required (format: YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	targetDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "Invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	startOfDay := targetDate
	endOfDay := targetDate.Add(24 * time.Hour)

	client, err := getOntDB(ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	measurements, err := client.GetMeasurementsByDateRange(startOfDay, endOfDay)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lastMeasurements, err := client.GetLastMeasurementBeforeDate(startOfDay)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	status := calculateOntStatusTotal(measurements, lastMeasurements)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func calculateOntStatus(measurements []models.Ont, lastMeasurements map[int]models.Ont, gponIdx int) models.OntStatusCount {
	activeOnts := make(map[int]bool)
	inactiveOnts := make(map[int]bool)
	errorOnts := make(map[int]bool)

	for _, m := range measurements {
		key := m.GponIdx*1000 + m.OntIdx
		if m.ControlRunStatus == 1 {
			activeOnts[key] = true
		}
	}

	for key, m := range lastMeasurements {
		if m.GponIdx != gponIdx {
			continue
		}
		if _, wasActive := activeOnts[key]; wasActive {
			continue
		}

		if m.ControlRunStatus == 2 {
			inactiveOnts[key] = true
		} else if m.ControlRunStatus == -1 {
			errorOnts[key] = true
		}
	}

	return models.OntStatusCount{
		Active:   len(activeOnts),
		Inactive: len(inactiveOnts),
		Error:    len(errorOnts),
	}
}

func calculateOntStatusTotal(measurements []models.Ont, lastMeasurements map[int]models.Ont) models.OntStatusCount {
	activeOnts := make(map[int]bool)
	inactiveOnts := make(map[int]bool)
	errorOnts := make(map[int]bool)

	for _, m := range measurements {
		key := m.GponIdx*1000 + m.OntIdx
		if m.ControlRunStatus == 1 {
			activeOnts[key] = true
		}
	}

	for key, m := range lastMeasurements {
		if _, wasActive := activeOnts[key]; wasActive {
			continue
		}

		if m.ControlRunStatus == 2 {
			inactiveOnts[key] = true
		} else if m.ControlRunStatus == -1 {
			errorOnts[key] = true
		}
	}

	return models.OntStatusCount{
		Active:   len(activeOnts),
		Inactive: len(inactiveOnts),
		Error:    len(errorOnts),
	}
}
