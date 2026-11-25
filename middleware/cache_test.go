package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func jsonHandler(calls *atomic.Int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"path":%q}`, r.URL.Path)
	})
}

func TestCacheReutilizaRespuestas(t *testing.T) {
	var calls atomic.Int32
	h := NewCache(CacheConfig{TTL: time.Minute, MaxBytes: 1 << 20})(jsonHandler(&calls))

	get := func(rawurl string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, rawurl, nil))
		return rec
	}

	first := get("/api/v1/traffic/10.0.0.1?initDate=a&endDate=b")
	if first.Code != http.StatusOK || first.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("primera petición: code=%d cache=%q", first.Code, first.Header().Get("X-Cache"))
	}

	// Mismo contenido con los parámetros en otro orden: debe ser un HIT.
	second := get("/api/v1/traffic/10.0.0.1?endDate=b&initDate=a")
	if second.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("se esperaba HIT, llegó %q", second.Header().Get("X-Cache"))
	}
	if second.Body.String() != first.Body.String() {
		t.Fatalf("cuerpo distinto: %q != %q", second.Body.String(), first.Body.String())
	}
	if calls.Load() != 1 {
		t.Fatalf("el handler se ejecutó %d veces, se esperaba 1", calls.Load())
	}
}

func TestCacheNoGuardaErroresNiSalud(t *testing.T) {
	var calls atomic.Int32
	failing := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	h := NewCache(CacheConfig{TTL: time.Minute, MaxBytes: 1 << 20})(failing)

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/traffic/x", nil))
	}
	if calls.Load() != 2 {
		t.Fatalf("un error no debe cachearse (handler %d veces)", calls.Load())
	}

	var healthCalls atomic.Int32
	health := NewCache(CacheConfig{TTL: time.Minute, MaxBytes: 1 << 20})(jsonHandler(&healthCalls))
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		health.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	}
	if healthCalls.Load() != 2 {
		t.Fatalf("la salud no debe cachearse (handler %d veces)", healthCalls.Load())
	}
}

func TestCacheExpira(t *testing.T) {
	var calls atomic.Int32
	h := NewCache(CacheConfig{TTL: 20 * time.Millisecond, MaxBytes: 1 << 20})(jsonHandler(&calls))

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/olt", nil))
		time.Sleep(30 * time.Millisecond)
	}
	if calls.Load() != 2 {
		t.Fatalf("la entrada no expiró (handler %d veces)", calls.Load())
	}
}

func TestCacheDesactivada(t *testing.T) {
	var calls atomic.Int32
	h := NewCache(CacheConfig{TTL: 0, MaxBytes: 1 << 20})(jsonHandler(&calls))

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/olt", nil))
	}
	if calls.Load() != 2 {
		t.Fatalf("con TTL=0 no debe cachear (handler %d veces)", calls.Load())
	}
}

func TestTTLParaRangosCerrados(t *testing.T) {
	c := &responseCache{ttl: time.Minute}

	closed := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traffic/x?endDate="+closed, nil)
	if got := c.ttlFor(req); got != closedRangeTTL {
		t.Fatalf("rango cerrado: TTL=%s, se esperaba %s", got, closedRangeTTL)
	}

	open := time.Now().UTC().Format(time.RFC3339)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/traffic/x?endDate="+open, nil)
	if got := c.ttlFor(req); got != time.Minute {
		t.Fatalf("rango abierto: TTL=%s, se esperaba 1m", got)
	}
}

func TestCacheDescartaRespuestasGrandes(t *testing.T) {
	var calls atomic.Int32
	big := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(make([]byte, maxCaptureBytes+1))
	})
	h := NewCache(CacheConfig{TTL: time.Minute, MaxBytes: 1 << 20})(big)

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/traffic/x", nil))
	}
	if calls.Load() != 2 {
		t.Fatalf("una respuesta enorme no debe cachearse (handler %d veces)", calls.Load())
	}
}
