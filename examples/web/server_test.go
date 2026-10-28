package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-router"
)

func TestFiberV3Pages(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en", "es"}
	cfg.Features.Themes = true
	cfg.Features.Widgets = true
	cfg.Cache.Enabled = false
	module, err := cms.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := setupDemoData(context.Background(), module, &cfg, filepath.Join("themes", "default")); err != nil {
		t.Fatal(err)
	}
	server, err := buildServer("./views")
	if err != nil {
		t.Fatal(err)
	}
	setupRoutes(server.Router(), module, &cfg, t.TempDir())
	server.Router().Static("/static", "./static")
	app := server.WrappedRouter()
	for _, tc := range []struct {
		path, contains string
		status         int
	}{
		{"/", "Welcome to go-cms", 200},
		{"/?locale=es", "go-cms", 200},
		{"/pages/about", "About", 200},
		{"/pages/about?locale=es", "go-cms", 200},
		{"/pages/missing", "Page Not Found", 200},
		{"/static/css/style.css", "{", 200},
		{"/not-a-route", "", 404},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res, err := app.Test(httptest.NewRequest("GET", tc.path, nil))
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != tc.status || !strings.Contains(string(body), tc.contains) {
				t.Fatalf("status=%d body=%s", res.StatusCode, body)
			}
		})
	}
	req := httptest.NewRequest("GET", "/switch-locale/es", nil)
	req.Header.Set("Referer", "http://localhost/pages/about?source=demo")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 302 || res.Header.Get("Location") != "/pages/about?locale=es" {
		t.Fatalf("redirect: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
}

func TestFiberV3StartupFailure(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	server, err := buildServer("./views")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := serve(ctx, server, listener.Addr().String(), time.Second); err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("startup error = %v", err)
	}
}

func TestFiberV3Shutdown(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		name := "drain"
		if deadline {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			server, err := buildServer("./views")
			if err != nil {
				t.Fatal(err)
			}
			started, release := make(chan struct{}), make(chan struct{})
			server.Router().Get("/slow", func(c router.Context) error { close(started); <-release; return c.SendString("complete") })
			server.Router().Get("/ready", func(c router.Context) error { return c.SendString("ready") })
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := listener.Addr().String()
			listener.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			timeout := 3 * time.Second
			if deadline {
				timeout = 100 * time.Millisecond
			}
			done := make(chan error, 1)
			go func() { done <- serve(ctx, server, addr, timeout) }()
			released := false
			defer func() {
				if !released {
					close(release)
				}
				shutdown, stop := context.WithTimeout(context.Background(), time.Second)
				defer stop()
				_ = server.Shutdown(shutdown)
			}()
			client := &http.Client{Timeout: 3 * time.Second}
			waitForServer(t, client, "http://"+addr+"/ready", done)
			request := make(chan error, 1)
			go func() {
				res, err := client.Get("http://" + addr + "/slow")
				if err == nil {
					defer res.Body.Close()
					var body []byte
					body, err = io.ReadAll(res.Body)
					if err == nil && string(body) != "complete" {
						err = errors.New("incomplete response")
					}
				}
				request <- err
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("request did not start")
			}
			cancel()
			if deadline {
				select {
				case err := <-done:
					if err == nil {
						t.Fatal("expected shutdown deadline")
					}
				case <-time.After(2 * time.Second):
					t.Fatal("shutdown exceeded bound")
				}
			} else {
				select {
				case err := <-done:
					t.Fatalf("shutdown did not drain: %v", err)
				case <-time.After(50 * time.Millisecond):
				}
			}
			close(release)
			released = true
			if err := <-request; err != nil && !deadline {
				t.Fatal(err)
			}
			if !deadline {
				select {
				case err := <-done:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(4 * time.Second):
					t.Fatal("shutdown did not finish")
				}
			}
		})
	}
}

func waitForServer(t *testing.T, client *http.Client, url string, done <-chan error) {
	t.Helper()
	limit := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		res, err := client.Get(url)
		if err == nil {
			res.Body.Close()
			if res.StatusCode == 200 {
				return
			}
		}
		select {
		case err := <-done:
			t.Fatalf("server stopped: %v", err)
		case <-limit:
			t.Fatal("server not ready")
		case <-ticker.C:
		}
	}
}
