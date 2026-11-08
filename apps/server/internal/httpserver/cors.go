package httpserver

import (
	"net/http"
	"strings"
)

// allowCrossOrigin lets pages on other origins call the API: the desktop
// app, which loads the web build from its own files, and the web build in
// development. This is safe because the API never uses cookies: a request
// is authorized only by a bearer token that a foreign page does not have,
// so credentials are never allowed cross-origin.
func allowCrossOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin == "" || !strings.HasPrefix(request.URL.Path, "/api/") {
			next.ServeHTTP(writer, request)
			return
		}
		headers := writer.Header()
		headers.Set("Access-Control-Allow-Origin", origin)
		headers.Add("Vary", "Origin")
		headers.Set("Access-Control-Expose-Headers", "Retry-After")
		if request.Method == http.MethodOptions && request.Header.Get("Access-Control-Request-Method") != "" {
			headers.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
			headers.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			headers.Set("Access-Control-Max-Age", "600")
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}
