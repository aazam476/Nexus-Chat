package httpserver

import (
	"net/http"
	"regexp"
	"time"
)

// MaximumRequestBody is the largest request body the server reads: room
// for an 8 MiB avatar or a 1 MiB backup blob in base64, with margin.
// Without a limit, a client could make a handler read without end.
const MaximumRequestBody = 16 << 20

// transferTime is how long an attachment upload or download may take: a
// file at the 4 GiB maximum over a slow connection.
const transferTime = time.Hour

// attachmentPath matches attachment uploads and downloads. Their bodies
// are checked against the instance's attachment limit by the attachments
// feature, as they stream to disk.
var attachmentPath = regexp.MustCompile(`^/api/v1/conversations/[^/]+/attachments(/[^/]+)?$`)

// limitRequestBodies stops reading a body after MaximumRequestBody bytes;
// the handler then fails to decode it and answers 400. Attachment
// transfers are limited by their own feature instead, and get an hour
// rather than the server's usual minute.
func limitRequestBodies(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if attachmentPath.MatchString(request.URL.Path) {
			deadline := time.Now().Add(transferTime)
			controller := http.NewResponseController(writer)
			// Test recorders cannot set deadlines; real connections can.
			_ = controller.SetReadDeadline(deadline)
			_ = controller.SetWriteDeadline(deadline)
		} else {
			request.Body = http.MaxBytesReader(writer, request.Body, MaximumRequestBody)
		}
		next.ServeHTTP(writer, request)
	})
}
