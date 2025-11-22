package web

import "net/http"

// Handler returns a placeholder UI handler. Real UI can replace this with static files or templates.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><h1>kokoa control plane</h1><p>Web UI coming soon.</p></body></html>`))
	})
}
