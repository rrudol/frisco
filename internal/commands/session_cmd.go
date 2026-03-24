package commands

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

// defaultLoginURL is the Frisco login page opened by the interactive browser login command.
const defaultLoginURL = "https://www.frisco.pl/login"

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage session (token, headers, user_id).",
	}
	cmd.AddCommand(newSessionFromCurlCmd(), newSessionShowCmd(), newSessionVerifyCmd(), newSessionLoginCmd(), newSessionRefreshTokenCmd())
	return cmd
}

func newSessionFromCurlCmd() *cobra.Command {
	var curlStr string
	c := &cobra.Command{
		Use:   "from-curl",
		Short: "Load session from curl command.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cd, err := session.ParseCurl(curlStr)
			if err != nil {
				return err
			}
			s, err := session.Load()
			if err != nil {
				return err
			}
			session.ApplyFromCurl(s, cd)
			if err := session.Save(s); err != nil {
				return err
			}
			_, _ = cmd.OutOrStdout().Write([]byte("Session saved from curl.\n"))
			return printJSON(map[string]any{
				"base_url":      s.BaseURL,
				"user_id":       s.UserID,
				"token_saved":   tokenSaved(s),
				"headers_saved": headerKeysSorted(s.Headers),
			})
		},
	}
	c.Flags().StringVar(&curlStr, "curl", "", "Full curl command in quotes.")
	_ = c.MarkFlagRequired("curl")
	return c
}

func newSessionShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current session (sensitive values redacted).",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			return printJSON(session.RedactedCopy(s))
		},
	}
}

func newSessionVerifyCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "verify",
		Short: "Verify session has token and user_id; GET cart must succeed.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			if session.TokenString(s) == "" {
				return errors.New(
					"No token in session. Use 'session from-curl' or 'session login'.",
				)
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
			_, err = httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(
				cmd.OutOrStdout(),
				"Session OK: cart API responded successfully.",
			)
			return nil
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

// tokenSaved reports whether the session contains a non-empty access token.
func tokenSaved(s *session.Session) bool {
	if s == nil || s.Token == nil {
		return false
	}
	if str, ok := s.Token.(string); ok {
		return str != ""
	}
	return true
}

// headerKeysSorted returns the header map keys in sorted order.
func headerKeysSorted(h map[string]string) []string {
	if len(h) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func newSessionRefreshTokenCmd() *cobra.Command {
	var refresh string
	c := &cobra.Command{
		Use:   "refresh-token",
		Short: "Refresh access token via refresh token.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			rt := refresh
			if rt == "" {
				rt = refreshTokenString(s)
			}
			if rt == "" {
				return errors.New("Missing refresh token. Use --refresh-token or load session with session from-curl.")
			}
			payload := map[string]any{
				"grant_type":    "refresh_token",
				"refresh_token": rt,
			}
			result, err := httpclient.RequestJSON(s, "POST", "/app/commerce/connect/token", httpclient.RequestOpts{
				Data:       payload,
				DataFormat: httpclient.FormatForm,
			})
			if err != nil {
				return err
			}
			saved := false
			expiresIn := any(nil)
			if m, ok := result.(map[string]any); ok {
				expiresIn = m["expires_in"]
				if at, ok := stringField(m["access_token"]); ok && at != "" {
					s.Token = at
					if s.Headers == nil {
						s.Headers = map[string]string{}
					}
					s.Headers["Authorization"] = "Bearer " + at
				}
				if nr, ok := stringField(m["refresh_token"]); ok && nr != "" {
					s.RefreshToken = nr
				}
				if err := session.Save(s); err != nil {
					return err
				}
				saved = true
			}
			return printJSON(map[string]any{
				"saved":               saved,
				"token_saved":         session.TokenString(s) != "",
				"refresh_token_saved": session.RefreshTokenString(s) != "",
				"expires_in":          expiresIn,
			})
		},
	}
	c.Flags().StringVar(&refresh, "refresh-token", "", "Optional refresh token (otherwise from session).")
	return c
}

func newSessionLoginCmd() *cobra.Command {
	var loginURL string
	var timeoutSec int

	c := &cobra.Command{
		Use: "login",
		Short: "Interactive browser login and automatic session save.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}

			baseURL := s.BaseURL
			if baseURL == "" {
				baseURL = session.DefaultBaseURL
			}
			if loginURL == "" {
				loginURL = defaultLoginURL
			}
			if _, err := url.ParseRequestURI(loginURL); err != nil {
				return fmt.Errorf("Invalid --login-url: %w", err)
			}
			if timeoutSec <= 0 {
				return errors.New("--timeout must be > 0")
			}

			type authCapture struct {
				AccessToken  string
				RefreshToken string
				UserID       string
				CookieHeader string
			}
			captured := authCapture{}
			var mu sync.Mutex

			allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("headless", false),
				chromedp.Flag("disable-gpu", false),
			)
			allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), allocOpts...)
			defer cancelAlloc()
			ctx, cancelCtx := chromedp.NewContext(allocCtx)
			defer cancelCtx()

			chromedp.ListenTarget(ctx, func(ev any) {
				mu.Lock()
				defer mu.Unlock()

				switch e := ev.(type) {
				case *network.EventRequestWillBeSent:
					if captured.UserID == "" {
						if uid := session.ExtractUserID(e.Request.URL); uid != "" {
							captured.UserID = uid
						}
					}
				case *network.EventRequestWillBeSentExtraInfo:
					if captured.AccessToken == "" {
						if token := bearerFromHeaders(e.Headers); token != "" {
							captured.AccessToken = token
						}
					}
					if cookie := headerStringValue(e.Headers, "Cookie"); cookie != "" {
						if captured.CookieHeader == "" {
							captured.CookieHeader = cookie
						}
						if captured.RefreshToken == "" {
							if rt := session.ExtractRefreshTokenFromHeaderValue(cookie); rt != "" {
								captured.RefreshToken = rt
							}
						}
					}
				case *network.EventResponseReceivedExtraInfo:
					if captured.RefreshToken == "" {
						if rt := refreshTokenFromHeaders(e.Headers); rt != "" {
							captured.RefreshToken = rt
						}
					}
				}
			})

			if err := chromedp.Run(ctx,
				network.Enable(),
				chromedp.Navigate(loginURL),
			); err != nil {
				return fmt.Errorf("Could not start login browser: %w", err)
			}
			_, _ = fmt.Fprintln(
				cmd.OutOrStdout(),
				"Browser opened. Log in manually and CLI will capture token and save session.",
			)

			deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
			var accessDetectedAt time.Time
			for time.Now().Before(deadline) {
				time.Sleep(1 * time.Second)
				mu.Lock()
				gotToken := captured.AccessToken != ""
				gotRefresh := captured.RefreshToken != ""
				mu.Unlock()
				if gotToken && accessDetectedAt.IsZero() {
					accessDetectedAt = time.Now()
				}
				if gotToken && gotRefresh {
					break
				}
				// Give refresh token a short extra window after access token appears.
				if gotToken && !accessDetectedAt.IsZero() && time.Since(accessDetectedAt) > 8*time.Second {
					break
				}
			}

			allCookies, err := network.GetCookies().Do(ctx)
			if err == nil && len(allCookies) > 0 {
				pairs := make([]string, 0, len(allCookies))
				for _, ck := range allCookies {
					if ck == nil || ck.Name == "" {
						continue
					}
					pairs = append(pairs, ck.Name+"="+ck.Value)
				}
				if len(pairs) > 0 {
					mu.Lock()
					captured.CookieHeader = strings.Join(pairs, "; ")
					if captured.RefreshToken == "" {
						if rt := session.ExtractRefreshTokenFromHeaderValue(captured.CookieHeader); rt != "" {
							captured.RefreshToken = rt
						}
					}
					mu.Unlock()
				}
			}

			mu.Lock()
			accessToken := captured.AccessToken
			refreshToken := captured.RefreshToken
			userID := captured.UserID
			cookieHeader := captured.CookieHeader
			mu.Unlock()

			if accessToken == "" {
				return errors.New("Access token not detected. Try again and after login open account/cart page to trigger API requests.")
			}

			s.BaseURL = baseURL
			s.Token = accessToken
			if s.Headers == nil {
				s.Headers = map[string]string{}
			}
			s.Headers["Authorization"] = "Bearer " + accessToken
			if cookieHeader != "" {
				s.Headers["Cookie"] = cookieHeader
			}
			if refreshToken != "" {
				s.RefreshToken = refreshToken
			}
			if userID != "" {
				s.UserID = userID
			}

			if err := session.Save(s); err != nil {
				return err
			}

			return printJSON(map[string]any{
				"saved":               true,
				"base_url":            s.BaseURL,
				"user_id":             s.UserID,
				"token_saved":         session.TokenString(s) != "",
				"refresh_token_saved": session.RefreshTokenString(s) != "",
				"cookie_saved":        s.Headers["Cookie"] != "",
			})
		},
	}

	c.Flags().StringVar(&loginURL, "login-url", defaultLoginURL, "Login start URL.")
	c.Flags().IntVar(&timeoutSec, "timeout", 180, "Maximum wait time for token (seconds).")
	return c
}

// bearerFromHeaders extracts the Bearer token value from a CDP network headers map.
func bearerFromHeaders(headers network.Headers) string {
	for k := range headers {
		if !strings.EqualFold(k, "authorization") {
			continue
		}
		value := strings.TrimSpace(fmt.Sprint(headers[k]))
		parts := strings.SplitN(value, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

// headerStringValue returns the trimmed value for the named header (case-insensitive).
func headerStringValue(headers network.Headers, name string) string {
	for k := range headers {
		if strings.EqualFold(k, name) {
			return strings.TrimSpace(fmt.Sprint(headers[k]))
		}
	}
	return ""
}

// refreshTokenFromHeaders scans CDP network headers for a Set-Cookie/Cookie value
// containing a refresh token and returns it if found.
func refreshTokenFromHeaders(headers network.Headers) string {
	for k := range headers {
		if strings.EqualFold(k, "set-cookie") || strings.EqualFold(k, "cookie") {
			raw := strings.TrimSpace(fmt.Sprint(headers[k]))
			if token := session.ExtractRefreshTokenFromHeaderValue(raw); token != "" {
				return token
			}
		}
	}
	return ""
}
