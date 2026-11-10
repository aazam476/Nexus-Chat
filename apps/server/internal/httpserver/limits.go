package httpserver

import "net/http"

// MaximumRequestBody is the largest request body the server reads: room
// for an 8 MiB avatar or a 1 MiB backup blob in base64, with margin.
// Without a limit, a client could make a handler read without end.
const MaximumRequestBody = 16 << 20

// limitRequestBodies stops reading a body after MaximumRequestBody bytes;
// the handler then fails to decode it and answers 400.
func limitRequestBodies(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(writer, request.Body, MaximumRequestBody)
		next.ServeHTTP(writer, request)
	})
}
