package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/aazam476/nexus-chat/apps/server/internal/config"
)

// healthcheckTimeout is how long `nexus-chat-server healthcheck` waits.
// Docker's own healthcheck timeout (set in the compose file) is longer.
const healthcheckTimeout = 3 * time.Second

// runHealthcheck implements `nexus-chat-server healthcheck`, which Docker
// runs inside the container. The image has no shell or curl, so the server
// binary checks itself. It returns the exit code: 0 healthy, 1 not.
func runHealthcheck(ctx context.Context, getenv config.Getenv, errorOutput io.Writer) int {
	port := config.DefaultHTTPPort
	if value := getenv("HTTP_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			_, _ = fmt.Fprintln(errorOutput, "healthcheck: HTTP_PORT is not a number")
			return 1
		}
		port = parsed
	}
	ctx, cancel := context.WithTimeout(ctx, healthcheckTimeout)
	defer cancel()
	url := fmt.Sprintf("http://127.0.0.1:%d/healthz", port)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		_, _ = fmt.Fprintln(errorOutput, "healthcheck:", err)
		return 1
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		_, _ = fmt.Fprintln(errorOutput, "healthcheck:", err)
		return 1
	}
	_ = response.Body.Close() // nothing useful can be done if closing fails
	if response.StatusCode != http.StatusOK {
		_, _ = fmt.Fprintln(errorOutput, "healthcheck: status", response.StatusCode)
		return 1
	}
	return 0
}
