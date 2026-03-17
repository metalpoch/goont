package handlers

import (
	"encoding/json"
	"fmt"
	"goont/storage"
	"net/http"
	"os"
	"path/filepath"
)

var dataDir string

func SetDataDir(dir string) {
	dataDir = dir
}

func getOltDB() (*storage.OltClient, error) {
	dbPath := filepath.Join(dataDir, "olt.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("OLT database not found")
	}
	client, err := storage.NewOltDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open OLT database: %w", err)
	}
	return client, nil
}

// GetAllOLT - Obtener todos los olt en medicion
func GetAllOLT(w http.ResponseWriter, r *http.Request) {
	client, err := getOltDB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	olts, err := client.GetInfoOLTs()
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

	client, err := getOltDB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	olt, err := client.GetOLTByID(ip)
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
