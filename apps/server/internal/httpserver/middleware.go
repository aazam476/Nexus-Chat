package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// statusRecorder remembers the status code a handler wrote, for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader records the status code and passes it on.
func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

// Unwrap lets http.ResponseController reach the underlying writer (needed
// later for WebSocket upgrades and streaming).
func (recorder *statusRecorder) Unwrap() http.ResponseWriter {
	return recorder.ResponseWriter
}

// logRequests logs one line per request. It deliberately logs only the
// method, path, status and duration: never headers, query strings or bodies,
// which can hold tokens or ciphertext.
func logRequests(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(recorder, request)
		logger.InfoContext(request.Context(), "http request",
			slog.String("method", request.Method),
			slog.String("path", request.URL.Path),
			slog.Int("status", recorder.status),
			slog.Duration("duration", time.Since(start)))
	})
}

// recoverPanics turns a panic in a handler into a 500 response and an error
// log. Code must not panic in request paths (code standards §8.2); this is
// the safety net that keeps one bug from crashing the whole server.
func recoverPanics(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				handlePanic(writer, request, recovered, logger)
			}
		}()
		next.ServeHTTP(writer, request)
	})
}

func handlePanic(writer http.ResponseWriter, request *http.Request, recovered any, logger *slog.Logger) {
	// http.ErrAbortHandler is how handlers intentionally abort a response;
	// net/http expects it to keep propagating.
	if err, isError := recovered.(error); isError && errors.Is(err, http.ErrAbortHandler) {
		panic(recovered)
	}
	logger.ErrorContext(request.Context(), "panic while handling request",
		slog.String("path", request.URL.Path), slog.Any("panic", recovered))
	http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// securityHeaders sets headers that are correct for every response. Page
// headers such as Content-Security-Policy come from the web client.
//
// Cross-Origin-Resource-Policy matches allowCrossOrigin: the API, avatars
// included, is meant for apps on other origins (the desktop app shows
// avatars from app://), while the web build's own files are for its pages
// only.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		headers := writer.Header()
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("Referrer-Policy", "no-referrer")
		headers.Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(request.URL.Path, "/api/") {
			headers.Set("Cross-Origin-Resource-Policy", "cross-origin")
		} else {
			headers.Set("Cross-Origin-Resource-Policy", "same-origin")
		}
		next.ServeHTTP(writer, request)
	})
}
