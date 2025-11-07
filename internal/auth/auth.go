package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	directory "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// GetClient retrieves a token, saves the token, then returns the generated client.
func GetClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	tok, err := getTokenFromWebWithLocalServer(config)
	if err != nil {
		log.Warn().Err(err).Msg("Falling back to manual OAuth flow")
		return getTokenFromCLI(config)
	}
	return tok
}

func getTokenFromWebWithLocalServer(config *oauth2.Config) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("unable to start local callback server: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://localhost:%d/", port)
	config.RedirectURL = redirectURL

	state := fmt.Sprintf("state-token-%d", time.Now().UnixNano())
	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))

	fmt.Printf("\nOpening browser for Google authorization...\nIf it does not open automatically, please visit:\n%v\n\n", authURL)
	openBrowser(authURL)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	srv := &http.Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		if e := query.Get("error"); e != "" {
			message := fmt.Sprintf("Authorization failed: %s", e)
			http.Error(w, message, http.StatusBadRequest)
			select {
			case errCh <- fmt.Errorf("%s", message):
			default:
			}
			return
		}

		if query.Get("state") != state {
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			return
		}

		code := query.Get("code")
		if code == "" {
			http.Error(w, "Missing authorization code", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, "<html><body><h1>Authentication complete</h1><p>You can close this tab and return to the terminal.</p></body></html>")

		select {
		case codeCh <- code:
		default:
		}
	})
	srv.Handler = mux

	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	var authCode string
	select {
	case authCode = <-codeCh:
	case err := <-errCh:
		return nil, err
	case <-time.After(2 * time.Minute):
		return nil, fmt.Errorf("timed out waiting for authorization response")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if shutdownErr := srv.Shutdown(ctx); shutdownErr != nil {
		log.Warn().Err(shutdownErr).Msg("Failed to cleanly shutdown OAuth callback server")
	}

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}

	return tok, nil
}

func getTokenFromCLI(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatal().Err(err).Msg("Unable to read authorization code")
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to retrieve token from web")
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	log.Info().Str("path", path).Msg("Saving credential file")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatal().Err(err).Str("path", path).Msg("Unable to cache oauth token")
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	if cmd == nil {
		return
	}

	if err := cmd.Start(); err != nil {
		log.Warn().Err(err).Str("url", url).Msg("Unable to open browser automatically")
	}
}

// GetCalendarService creates and returns a Google Calendar service
func GetCalendarService(credentialsFile string) (*calendar.Service, error) {
	b, err := ioutil.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	// We need both calendar.readonly and directory.group.member.readonly scopes
	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope, directory.AdminDirectoryGroupMemberReadonlyScope, directory.AdminDirectoryGroupReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	client := GetClient(config)

	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Calendar client: %v", err)
	}

	return srv, nil
}

// GetDirectoryService creates and returns a Google Directory service
func GetDirectoryService(credentialsFile string) (*directory.Service, error) {
	b, err := ioutil.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	// We need both calendar.readonly and directory.group.member.readonly scopes
	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope, directory.AdminDirectoryGroupMemberReadonlyScope, directory.AdminDirectoryGroupReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	client := GetClient(config)

	srv, err := directory.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Directory client: %v", err)
	}

	return srv, nil
}
