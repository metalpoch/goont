package middleware

import (
	"bytes"
	"container/list"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CacheConfig define el comportamiento de la caché en memoria.
type CacheConfig struct {
	// TTL es cuánto tiempo se reutiliza una respuesta.
	TTL time.Duration
	// MaxBytes acota la memoria total ocupada por las respuestas guardadas.
	MaxBytes int64
}

// CacheConfigFromEnv lee GOONT_CACHE_TTL (segundos, 0 desactiva) y
// GOONT_CACHE_MB (MiB). Los valores inválidos se ignoran.
func CacheConfigFromEnv() CacheConfig {
	cfg := CacheConfig{
		TTL:      60 * time.Second,
		MaxBytes: 64 << 20,
	}

	if v := os.Getenv("GOONT_CACHE_TTL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.TTL = time.Duration(n) * time.Second
		}
	}
	if v := os.Getenv("GOONT_CACHE_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxBytes = int64(n) << 20
		}
	}

	return cfg
}

type cacheEntry struct {
	key         string
	body        []byte
	contentType string
	stored      time.Time
	ttl         time.Duration
}

// closedRangeTTL es el tiempo que se guarda un rango de fechas ya cerrado:
// sus datos no cambian, así que no tiene sentido volver a consultarlos.
const closedRangeTTL = time.Hour

type responseCache struct {
	ttl      time.Duration
	maxBytes int64

	mu      sync.Mutex
	entries map[string]*list.Element
	order   *list.List // del más reciente (front) al más antiguo (back)
	bytes   int64
}

// NewCache devuelve un middleware que guarda en memoria las respuestas JSON
// correctas de la API. Con TTL <= 0 devuelve un middleware transparente.
//
// Es una caché por proceso: suficiente mientras haya un único contenedor del
// servidor. Con varias réplicas haría falta una caché compartida (Redis).
func NewCache(cfg CacheConfig) func(http.Handler) http.Handler {
	if cfg.TTL <= 0 || cfg.MaxBytes <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	c := &responseCache{
		ttl:      cfg.TTL,
		maxBytes: cfg.MaxBytes,
		entries:  make(map[string]*list.Element),
		order:    list.New(),
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || !cacheablePath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			key := cacheKey(r)
			if entry, age, ok := c.get(key); ok {
				w.Header().Set("X-Cache", "HIT")
				w.Header().Set("Age", strconv.Itoa(int(age.Seconds())))
				w.Header().Set("Content-Type", entry.contentType)
				w.Header().Set("Content-Length", strconv.Itoa(len(entry.body)))
				_, _ = w.Write(entry.body)
				return
			}

			w.Header().Set("X-Cache", "MISS")
			ttl := c.ttlFor(r)

			// El cuerpo se envía al cliente al mismo tiempo que se copia para
			// la caché, sin acumular respuestas gigantes en memoria.
			cw := &captureWriter{ResponseWriter: w}
			next.ServeHTTP(cw, r)

			if cw.status != http.StatusOK || cw.overflow || cw.body.Len() == 0 {
				return
			}
			contentType := cw.Header().Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				return
			}
			body := cw.body.Bytes()
			if int64(len(body)) > c.maxBytes {
				return
			}
			c.set(key, body, contentType, ttl)
		})
	}
}

// cacheablePath excluye salud y la raíz: son baratos y conviene que reflejen
// el estado real del proceso en cada petición.
func cacheablePath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/") && path != "/api/v1/health"
}

// cacheKey normaliza el orden de los parámetros para que dos URLs equivalentes
// compartan entrada.
func cacheKey(r *http.Request) string {
	return r.URL.EscapedPath() + "?" + r.URL.Query().Encode()
}

func (c *responseCache) get(key string) (*cacheEntry, time.Duration, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[key]
	if !ok {
		return nil, 0, false
	}
	entry := elem.Value.(*cacheEntry)
	now := time.Now()
	age := now.Sub(entry.stored)
	if age >= entry.ttl {
		c.remove(elem)
		return nil, 0, false
	}
	c.order.MoveToFront(elem)
	return entry, age, true
}

// ttlFor usa un TTL largo cuando el rango pedido ya terminó: los datos
// históricos no cambian y el escritorio repite mucho esos bloques.
func (c *responseCache) ttlFor(r *http.Request) time.Duration {
	end := r.URL.Query().Get("endDate")
	if end == "" {
		return c.ttl
	}
	t, err := time.Parse(time.RFC3339, end)
	if err != nil {
		return c.ttl
	}
	if time.Since(t) > 10*time.Minute && c.ttl < closedRangeTTL {
		return closedRangeTTL
	}
	return c.ttl
}

func (c *responseCache) set(key string, body []byte, contentType string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.entries[key]; ok {
		c.remove(elem)
	}

	entry := &cacheEntry{
		key:         key,
		body:        body,
		contentType: contentType,
		stored:      time.Now(),
		ttl:         ttl,
	}
	elem := c.order.PushFront(entry)
	c.entries[key] = elem
	c.bytes += int64(len(body))

	for c.bytes > c.maxBytes && c.order.Len() > 0 {
		c.remove(c.order.Back())
	}
}

func (c *responseCache) remove(elem *list.Element) {
	if elem == nil {
		return
	}
	entry := elem.Value.(*cacheEntry)
	c.order.Remove(elem)
	delete(c.entries, entry.key)
	c.bytes -= int64(len(entry.body))
}

// captureWriter reenvía la respuesta al cliente y guarda una copia hasta un
// límite; si la respuesta es demasiado grande deja de copiar y no se cachea.
type captureWriter struct {
	http.ResponseWriter
	status   int
	overflow bool
	body     bytes.Buffer
}

func (w *captureWriter) WriteHeader(code int) {
	if w.status != 0 {
		return
	}
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *captureWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if !w.overflow {
		if w.body.Len()+len(p) > maxCaptureBytes {
			w.overflow = true
			w.body.Reset()
		} else {
			_, _ = w.body.Write(p)
		}
	}
	return w.ResponseWriter.Write(p)
}

// maxCaptureBytes limita la copia por respuesta; 8 MiB es holgado para las
// series que devuelve la API y evita que una respuesta anómala llene la caché.
const maxCaptureBytes = 8 << 20
