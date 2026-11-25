package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
)

// retryAfterSeconds is the hint sent with 503 responses: the client should
// back off instead of retrying immediately and making pool exhaustion worse.
const retryAfterSeconds = 5

// writeServerError logs the full error server-side and answers with a generic
// message. SQLSTATE codes, query text and connection details must never reach
// the API consumer.
//
// Mapping:
//   - 53300 too_many_connections -> 503 + Retry-After (Postgres pool full);
//   - an inner deadline (e.g. waiting for a pooled connection) -> 503;
//   - the request context itself expiring -> 504;
//   - 57014 statement timeout -> 504;
//   - anything else -> 500.
func writeServerError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "53300": // too_many_connections
			writeServiceUnavailable(w)
			return
		case "57014": // query_canceled (statement_timeout)
			writeGatewayTimeout(w)
			return
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		if r.Context().Err() == nil {
			// The deadline came from an operation inside the handler, not
			// from the request itself: typically the pool could not hand
			// out a connection in time.
			writeServiceUnavailable(w)
			return
		}
		writeGatewayTimeout(w)
		return
	}

	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func writeServiceUnavailable(w http.ResponseWriter) {
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	http.Error(w, "database temporarily unavailable", http.StatusServiceUnavailable)
}

func writeGatewayTimeout(w http.ResponseWriter) {
	http.Error(w, "request timed out", http.StatusGatewayTimeout)
}
