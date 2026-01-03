package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/tasks/v1"
)

var scopes = []string{
	tasks.TasksScope,
	calendar.CalendarScope,
}

func Client(ctx context.Context, credentialsPath, tokenPath string) (*http.Client, error) {
	// #nosec G304 -- credentials path is user-configured
	creds, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	config, err := google.ConfigFromJSON(creds, scopes...)
	if err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	ok, err := tokenFromFile(tokenPath)
	if err == nil {
		return config.Client(ctx, ok), nil
	}

	tok, err := getTokenFromWeb(config)
	if err != nil {
		return nil, err
	}
	if err := saveToken(tokenPath, tok); err != nil {
		return nil, err
	}
	return config.Client(ctx, tok), nil
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return getTokenFromWebManual(config)
	}
	redirectURL := fmt.Sprintf("http://%s/callback", ln.Addr().String())
	cfg := *config
	cfg.RedirectURL = redirectURL

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/callback" {
				http.NotFound(w, r)
				return
			}
			if r.URL.Query().Get("state") != "state-token" {
				http.Error(w, "Invalid state", http.StatusBadRequest)
				return
			}
			code := r.URL.Query().Get("code")
			if code == "" {
				http.Error(w, "Missing code", http.StatusBadRequest)
				return
			}
			_, _ = fmt.Fprintln(w, "Auth complete. You can close this tab and return to the CLI.")
			codeCh <- code
		}),
	}

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	authURL := cfg.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Println()
	fmt.Println("Authorize justdoit in your browser:")
	fmt.Printf("  %s\n", clickableLink("Open authorization link", authURL))
	if os.Getenv("JUSTDOIT_SHOW_AUTH_URL") != "" {
		fmt.Printf("  URL: %s\n", authURL)
	} else {
		fmt.Println("  (If it doesn't open, re-run with JUSTDOIT_SHOW_AUTH_URL=1)")
	}
	fmt.Println("Waiting for authorization...")

	select {
	case code := <-codeCh:
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = srv.Shutdown(context.Background())
		return cfg.Exchange(ctx, code)
	case err := <-errCh:
		_ = srv.Shutdown(context.Background())
		return nil, err
	case <-time.After(5 * time.Minute):
		_ = srv.Shutdown(context.Background())
		return nil, fmt.Errorf("authorization timed out")
	}
}

func getTokenFromWebManual(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Open this URL in your browser and paste the authorization code:\n%v\n", authURL)
	var code string
	if _, err := fmt.Scan(&code); err != nil {
		return nil, fmt.Errorf("read authorization code: %w", err)
	}
	return config.Exchange(context.Background(), code)
}

func clickableLink(text, url string) string {
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}

func tokenFromFile(path string) (*oauth2.Token, error) {
	// #nosec G304 -- token path is user-configured
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	var tok oauth2.Token
	if err := json.NewDecoder(file).Decode(&tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func saveToken(path string, token *oauth2.Token) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// #nosec G304 -- token path is user-configured
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return json.NewEncoder(file).Encode(token)
}
