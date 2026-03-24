package commands

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

const defaultLoginURL = "https://www.frisco.pl/login"

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Autoryzacja i odświeżanie tokena.",
	}
	cmd.AddCommand(newAuthRefreshTokenCmd(), newAuthLoginCmd())
	return cmd
}

func newAuthRefreshTokenCmd() *cobra.Command {
	var refresh string
	c := &cobra.Command{
		Use:   "refresh-token",
		Short: "Odśwież access token przez refresh token.",
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
				return fmt.Errorf("Brak refresh tokena. Podaj --refresh-token albo wczytaj go przez session from-curl.")
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
			if m, ok := result.(map[string]any); ok {
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
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&refresh, "refresh-token", "", "Opcjonalny refresh token (inaczej z sesji).")
	return c
}

func newAuthLoginCmd() *cobra.Command {
	var loginURL string
	var timeoutSec int

	c := &cobra.Command{
		Use:   "login",
		Short: "Interaktywne logowanie przez przeglądarkę i automatyczny zapis sesji.",
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
				return fmt.Errorf("Niepoprawny --login-url: %w", err)
			}
			if timeoutSec <= 0 {
				return fmt.Errorf("--timeout musi być > 0")
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
				switch e := ev.(type) {
				case *network.EventRequestWillBeSent:
					reqURL := e.Request.URL
					mu.Lock()
					if captured.UserID == "" {
						if uid := session.ExtractUserID(reqURL); uid != "" {
							captured.UserID = uid
						}
					}
					mu.Unlock()
				case *network.EventRequestWillBeSentExtraInfo:
					mu.Lock()
					if captured.AccessToken == "" {
						if token := bearerFromHeaders(e.Headers); token != "" {
							captured.AccessToken = token
						}
					}
					if captured.CookieHeader == "" {
						if cookie := headerStringValue(e.Headers, "Cookie"); cookie != "" {
							captured.CookieHeader = cookie
							if captured.RefreshToken == "" {
								if rt := session.ExtractRefreshTokenFromCookie(cookie); rt != "" {
									captured.RefreshToken = rt
								}
							}
						}
					}
					mu.Unlock()
				}
			})

			if err := chromedp.Run(ctx,
				network.Enable(),
				chromedp.Navigate(loginURL),
			); err != nil {
				return fmt.Errorf("Nie udało się uruchomić przeglądarki logowania: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Otwarta przeglądarka. Zaloguj się ręcznie, CLI wykryje token i zapisze sesję.")

			deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
			for time.Now().Before(deadline) {
				time.Sleep(1 * time.Second)
				mu.Lock()
				gotToken := captured.AccessToken != ""
				mu.Unlock()
				if gotToken {
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
						if rt := session.ExtractRefreshTokenFromCookie(captured.CookieHeader); rt != "" {
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
				return fmt.Errorf(
					"Nie wykryto access tokena. Spróbuj ponownie i po zalogowaniu przejdź do strony konta lub koszyka, żeby wymusić zapytania API",
				)
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

	c.Flags().StringVar(&loginURL, "login-url", defaultLoginURL, "URL startowy do logowania.")
	c.Flags().IntVar(&timeoutSec, "timeout", 180, "Maksymalny czas oczekiwania na token (sekundy).")
	return c
}

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

func headerStringValue(headers network.Headers, name string) string {
	for k := range headers {
		if strings.EqualFold(k, name) {
			return strings.TrimSpace(fmt.Sprint(headers[k]))
		}
	}
	return ""
}
