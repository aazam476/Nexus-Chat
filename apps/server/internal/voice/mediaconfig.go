package voice

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Ports used inside the media containers. They match
// infra/compose/docker-compose.yml.
const (
	LiveKitHTTPPort = 7880 // signaling WebSocket and API (internal only)
	LiveKitTCPPort  = 7881 // ICE over TCP, for clients behind strict firewalls
	LiveKitUDPPort  = 7882 // all WebRTC media multiplexed on one UDP port
	TURNPort        = 3478 // LiveKit's built-in TURN (SFU mode)
	CoturnPort      = 3479 // coturn (P2P mode); differs so both can run together
)

// MediaSecrets are the keys the media servers need.
type MediaSecrets struct {
	LiveKitAPIKey    string
	LiveKitAPISecret string
	TURNSecret       string
}

// WriteMediaConfig writes livekit/livekit.yaml and coturn/turnserver.conf
// under dataDirectory. Both containers mount their folder read-only and run
// as the same user as the server, so the files can stay owner-only even
// though they contain secrets.
func WriteMediaConfig(dataDirectory string, secrets MediaSecrets) error {
	if err := validateSecrets(secrets); err != nil {
		return err
	}
	files := map[string]string{
		filepath.Join("livekit", "livekit.yaml"):   liveKitConfig(secrets),
		filepath.Join("coturn", "turnserver.conf"): coturnConfig(secrets),
	}
	for name, contents := range files {
		path := filepath.Join(dataDirectory, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

// validateSecrets refuses values that could break out of the config file
// syntax. Generated secrets are base32 and always pass; this guards against
// a hand-edited secrets file.
func validateSecrets(secrets MediaSecrets) error {
	for name, value := range map[string]string{
		"LiveKit API key": secrets.LiveKitAPIKey, "LiveKit API secret": secrets.LiveKitAPISecret, "TURN secret": secrets.TURNSecret,
	} {
		if value == "" || strings.ContainsFunc(value, func(r rune) bool { return !isAlphanumeric(r) }) {
			return fmt.Errorf("%s must be non-empty and contain only letters and digits", name)
		}
	}
	return nil
}

func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// liveKitConfig follows LiveKit's config.yaml reference. use_external_ip
// makes LiveKit discover the host's public address with STUN, so clients
// outside the server's network can reach the media port.
func liveKitConfig(secrets MediaSecrets) string {
	return fmt.Sprintf(`# Written by nexus-chat-server on every start. Do not edit: changes are overwritten.
port: %d
rtc:
  tcp_port: %d
  udp_port: %d
  use_external_ip: true
keys:
  %s: %s
turn:
  enabled: true
  udp_port: %d
logging:
  level: info
`, LiveKitHTTPPort, LiveKitTCPPort, LiveKitUDPPort, secrets.LiveKitAPIKey, secrets.LiveKitAPISecret, TURNPort)
}

// coturnConfig enables time-limited credentials derived from the shared
// secret (the TURN REST API scheme) and refuses to relay to private and
// loopback addresses, so the TURN server cannot be used to reach machines
// on the server's internal network.
func coturnConfig(secrets MediaSecrets) string {
	return fmt.Sprintf(`# Written by nexus-chat-server on every start. Do not edit: changes are overwritten.
listening-port=%d
use-auth-secret
static-auth-secret=%s
realm=nexus-chat
fingerprint
no-cli
no-tls
no-dtls
no-multicast-peers
min-port=49160
max-port=49200
denied-peer-ip=0.0.0.0-0.255.255.255
denied-peer-ip=10.0.0.0-10.255.255.255
denied-peer-ip=100.64.0.0-100.127.255.255
denied-peer-ip=127.0.0.0-127.255.255.255
denied-peer-ip=169.254.0.0-169.254.255.255
denied-peer-ip=172.16.0.0-172.31.255.255
denied-peer-ip=192.168.0.0-192.168.255.255
denied-peer-ip=::1
denied-peer-ip=fc00::-fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff
denied-peer-ip=fe80::-febf:ffff:ffff:ffff:ffff:ffff:ffff:ffff
`, CoturnPort, secrets.TURNSecret)
}
