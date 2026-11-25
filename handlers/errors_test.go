package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func requestWithContext(t *testing.T, ctx context.Context) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traffic/10.0.0.1", nil)
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	return req
}

func TestWriteServerError_PoolExhaustedIs503(t *testing.T) {
	rec := httptest.NewRecorder()
	err := &pgconn.PgError{Code: "53300", Message: "sorry, too many clients already"}
	writeServerError(rec, requestWithContext(t, nil), err)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, se esperaba 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "5" {
		t.Fatalf("Retry-After = %q, se esperaba 5", got)
	}
	if strings.Contains(rec.Body.String(), "53300") {
		t.Fatalf("la respuesta filtra el SQLSTATE: %q", rec.Body.String())
	}
}

func TestWriteServerError_InnerDeadlineIs503(t *testing.T) {
	rec := httptest.NewRecorder()
	// Contexto de la peticion vivo: el deadline vino de una operacion interna
	// (p. ej. esperar una conexion del pool).
	writeServerError(rec, requestWithContext(t, context.Background()), context.DeadlineExceeded)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, se esperaba 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "5" {
		t.Fatalf("Retry-After = %q, se esperaba 5", got)
	}
}

func TestWriteServerError_RequestTimeoutIs504(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rec := httptest.NewRecorder()
	writeServerError(rec, requestWithContext(t, ctx), context.DeadlineExceeded)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("code = %d, se esperaba 504", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Fatalf("504 no debe llevar Retry-After, got %q", got)
	}
}

func TestWriteServerError_StatementTimeoutIs504(t *testing.T) {
	rec := httptest.NewRecorder()
	err := &pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout"}
	writeServerError(rec, requestWithContext(t, nil), err)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("code = %d, se esperaba 504", rec.Code)
	}
}

func TestWriteServerError_HidesInternalDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	err := errors.New(`query olt traffic: ERROR: relation "ont_measurements" does not exist (SQLSTATE 42P01)`)
	writeServerError(rec, requestWithContext(t, nil), err)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, se esperaba 500", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "SQLSTATE") || strings.Contains(body, "ont_measurements") {
		t.Fatalf("la respuesta filtra detalles internos: %q", body)
	}
	if !strings.Contains(body, "internal server error") {
		t.Fatalf("mensaje generico inesperado: %q", body)
	}
}
