package cli

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// refreshLoop asks fzf to reload the list while the popup is open. fzf keeps
// the query and the current item across a reload, so the view stays put while
// badges and ages update. The loop dies with the picker; nothing outlives it.
func refreshLoop(sock, action string, stop <-chan struct{}) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		},
	}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			req, err := http.NewRequest(http.MethodPost, "http://localhost/", strings.NewReader(action))
			if err != nil {
				continue
			}
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
	}
}
