// Package linkpreview is the instance's optional link preview proxy (spec
// §4.5, docs/e2ee-design.md §8). Senders' native and desktop apps build
// previews themselves; browsers cannot read other sites, so the web app may
// ask the instance to fetch a page's title, description and image instead.
// The admin turns this on, and users are told the instance then sees the
// links they preview. Nothing is stored.
//
// Every fetch goes through a dialer that refuses private, loopback,
// link-local and other internal addresses after DNS resolution, so the
// proxy cannot be used to reach the instance's own network. Start reading
// at service.go.
package linkpreview
