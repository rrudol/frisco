package commands

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/session"
)

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: tr("Manage session (token, headers, user_id).", "Zarządzanie sesją (token, headers, user_id)."),
	}
	cmd.AddCommand(newSessionFromCurlCmd(), newSessionShowCmd())
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
