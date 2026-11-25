package middleware

import (
	"context"
	"net/http"
	"time"
)

// Timeout acota el tiempo total de una petición. Las consultas de tráfico
// pueden tardar decenas de segundos, así que el plazo debe ser mayor que el
// del cliente (85 s en el escritorio) pero finito: si el cliente se
// desconecta, el contexto de la petición ya cancela la consulta en Postgres.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if d <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
