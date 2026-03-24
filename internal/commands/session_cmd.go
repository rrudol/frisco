package commands

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: tr("Manage session (token, headers, user_id).", "Zarządzanie sesją (token, headers, user_id)."),
	}
	cmd.AddCommand(newSessionFromCurlCmd(), newSessionShowCmd(), newSessionVerifyCmd())
	return cmd
}

func newSessionFromCurlCmd() *cobra.Command {
	var curlStr string
	c := &cobra.Command{
		Use:   "from-curl",
		Short: tr("Load session from curl command.", "Wczytaj sesję z komendy curl."),
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
			_, _ = cmd.OutOrStdout().Write([]byte(tr("Session saved from curl.\n", "Zapisano sesję na podstawie curl.\n")))
			return printJSON(map[string]any{
				"base_url":      s.BaseURL,
				"user_id":       s.UserID,
				"token_saved":   tokenSaved(s),
				"headers_saved": headerKeysSorted(s.Headers),
			})
		},
	}
	c.Flags().StringVar(&curlStr, "curl", "", tr("Full curl command in quotes.", "Cała komenda curl w cudzysłowie."))
	_ = c.MarkFlagRequired("curl")
	return c
}

func newSessionShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: tr("Show current session (sensitive values redacted).", "Pokaż aktualną sesję (wrażliwe dane ukryte)."),
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
		Short: tr(
			"Verify session has token and user_id; GET cart must succeed.",
			"Sprawdź sesję: token i user_id; GET koszyka musi się udać.",
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			if session.TokenString(s) == "" {
				return errors.New(
					tr(
						"No token in session. Use session from-curl or auth login.",
						"Brak tokenu w sesji. Użyj session from-curl albo auth login.",
					),
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
				tr(
					"Session OK: cart API responded successfully.",
					"Sesja OK: API koszyka odpowiedziało poprawnie.",
				),
			)
			return nil
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func tokenSaved(s *session.Session) bool {
	if s == nil || s.Token == nil {
		return false
	}
	if str, ok := s.Token.(string); ok {
		return str != ""
	}
	return true
}

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
