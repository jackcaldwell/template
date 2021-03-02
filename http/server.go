package http

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"log"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"template"
	"time"
)

// ShutdownTimeout is the time given for outstanding requests to finish before shutdown.
const ShutdownTimeout = 1 * time.Second

type Server struct {
	ln     net.Listener
	server *http.Server
	router *mux.Router
	sc     *securecookie.SecureCookie

	// Bind address & domain for the server's listener.
	// If domain is specified, server is run on TLS using acme/autocert.
	Addr   string
	Domain string

	// Keys used for secure cookie encryption.
	HashKey  string
	BlockKey string

	// GitHub OAuth settings.
	GitHubClientID     string
	GitHubClientSecret string

	// Services
	AuthService    template.AuthService
	UserService    template.UserService

	HelloService template.HelloService
}

func NewServer() *Server {
	s := &Server{
		router: mux.NewRouter(),
		server: &http.Server{},
	}

	// Our router is wrapped by another function handler to perform some
	// middleware-like tasks that cannot be performed by actual middleware.
	// This includes changing route paths for JSON endpoints & overridding methods.
	s.server.Handler = http.HandlerFunc(s.serveHTTP)

	return s
}

// UseTLS returns true if the cert & key file are specified.
func (s *Server) UseTLS() bool {
	return s.Domain != ""
}

func (s *Server) Open() (err error) {
	// Assign all the
	s.registerRoutes()

	// Initialize our secure cookie with our encryption keys.
	if err := s.openSecureCookie(); err != nil {
		return err
	}

	// Validate GitHub OAuth settings.
	if s.GitHubClientID == "" {
		return fmt.Errorf("github client id required")
	} else if s.GitHubClientSecret == "" {
		return fmt.Errorf("github client secret required")
	}

	// Open a listener on our bind address.
	if s.Domain != "" {
		s.ln = autocert.NewListener(s.Domain)
	} else {
		if s.ln, err = net.Listen("tcp", s.Addr); err != nil {
			return err
		}
	}

	// Begin serving requests on the listener. We use Serve() instead of
	// ListenAndServe() because it allows us to check for listen errors (such
	// as trying to use an already open port) synchronously.
	err = s.server.Serve(s.ln)
	if err != nil {
		return err
	}

	return nil
}

// openSecureCookie validates & decodes the block & hash key and initializes
// our secure cookie implementation.
func (s *Server) openSecureCookie() error {
	// Ensure hash & block key are set.
	if s.HashKey == "" {
		return fmt.Errorf("hash key required")
	} else if s.BlockKey == "" {
		return fmt.Errorf("block key required")
	}

	// Decode from hex to byte slices.
	hashKey, err := hex.DecodeString(s.HashKey)
	if err != nil {
		return fmt.Errorf("invalid hash key")
	}
	blockKey, err := hex.DecodeString(s.BlockKey)
	if err != nil {
		return fmt.Errorf("invalid block key")
	}

	// Initialize cookie management & encode our cookie data as JSON.
	s.sc = securecookie.New(hashKey, blockKey)
	s.sc.SetSerializer(securecookie.JSONEncoder{})

	return nil
}

// Close gracefully shuts down the server.
func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// OAuth2Config returns the GitHub OAuth2 configuration.
func (s *Server) OAuth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.GitHubClientID,
		ClientSecret: s.GitHubClientSecret,
		Scopes:       []string{},
		Endpoint:     github.Endpoint,
	}
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	// Override method for forms passing "_method" value.
	if r.Method == http.MethodPost {
		switch v := r.PostFormValue("_method"); v {
		case http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete:
			r.Method = v
		}
	}

	// Override content-type for certain extensions.
	// This allows us to easily cURL API endpoints with a ".json"
	// extension instead of having to explicitly set Content-type & Accept headers.
	// The extension is removed so it doesn't appear in the routes.
	if path.Ext(r.URL.Path) == ".json" {
		r.Header.Set("Accept", "application/json")
		r.Header.Set("Content-type", "application/json")
		r.URL.Path = strings.TrimSuffix(r.URL.Path, ".json")
	}

	// Allow CORS
	allowedHeaders := handlers.AllowedHeaders([]string{"Content-Type", "Authorization"})
	allowedOrigins := handlers.AllowedOrigins([]string{"http://localhost:3000"})
	allowedMethods := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS", "DELETE"})

	// Delegate remaining HTTP handling to the gorilla router.
	handlers.CORS(
		allowedOrigins,
		allowedHeaders,
		allowedMethods,
		handlers.AllowCredentials(),
	)(s.router).ServeHTTP(w, r)
}

// session returns session data from the secure cookie.
func (s *Server) session(r *http.Request) (Session, error) {
	// Read session data from cookie.
	// If it returns an error then simply return an empty session.
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return Session{}, nil
	}

	// Decode session data into a Session object & return.
	var session Session
	if err := s.UnmarshalSession(cookie.Value, &session); err != nil {
		return Session{}, err
	}
	return session, nil
}

// setSession creates a secure cookie with session data.
func (s *Server) setSession(w http.ResponseWriter, session Session) error {
	// Encode session data to JSON.
	buf, err := s.MarshalSession(session)
	if err != nil {
		return err
	}

	// Write cookie to HTTP response.
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    buf,
		Path:     "/",
		Expires:  time.Now().Add(30 * 24 * time.Hour),
		Secure:   s.UseTLS(),
		HttpOnly: true,
	})
	return nil
}

// MarshalSession encodes session data to string.
// This is exported to allow the unit tests to generate fake sessions.
func (s *Server) MarshalSession(session Session) (string, error) {
	return s.sc.Encode(SessionCookieName, session)
}

// UnmarshalSession decodes session data into a Session object.
// This is exported to allow the unit tests to generate fake sessions.
func (s *Server) UnmarshalSession(data string, session *Session) error {
	return s.sc.Decode(SessionCookieName, data, &session)
}

// ListenAndServeTLSRedirect runs an HTTP server on port 80 to redirect users
// to the TLS-enabled port 443 server.
func ListenAndServeTLSRedirect(domain string) error {
	return http.ListenAndServe(":80", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://"+domain, http.StatusFound)
	}))
}

// ListenAndServeDebug runs an HTTP server with /debug endpoints (e.g. pprof, vars).
func ListenAndServeDebug() error {
	h := http.NewServeMux()
	return http.ListenAndServe(":6060", h)
}

// authenticate is middleware for loading session data from a cookie.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read session from secure cookie.
		session, _ := s.session(r)

		// Read user, if available. Ignore if fetching assets.
		if session.UserID != 0 {
			if user, err := s.UserService.GetUserByID(r.Context(), session.UserID); err != nil {
				log.Printf("cannot find session user: id=%d err=%s", session.UserID, err)
			} else {
				r = r.WithContext(template.NewContextWithUser(r.Context(), user))
			}
		}

		next.ServeHTTP(w, r)
	})
}

// paginationFromQuery gets the query params required for pagination from a http.Request. Uses default
// values if parsing fails.
func paginationFromQuery(r *http.Request) (int, int) {
	p, err := strconv.Atoi(r.URL.Query().Get("page"))

	if p <= 0 || err != nil {
		p = 1
	}

	l, err := strconv.Atoi(r.URL.Query().Get("limit"))

	switch {
	case err != nil:
		l = 10
	case l > 100:
		l = 100
	case l <= 0:
		l = 10
	}

	return l, p
}

func dateFromQuery(source string) (*time.Time, error) {
	if source == "" {
		return nil, nil
	}
	layout := "2006-01-02T15:04:05Z"
	date, err := time.Parse(layout, source)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

func stringFromQuery(source string) *string {
	if source == "" {
		return nil
	}
	return &source
}

func intFromQuery(source string) (*int, error) {
	if source == "" {
		return nil, nil
	}
	res, err := strconv.Atoi(source)

	return &res, err
}
