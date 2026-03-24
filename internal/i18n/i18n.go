package i18n

import (
	"strings"
	"sync"
)

type Lang string

const (
	EN Lang = "en"
	PL Lang = "pl"
)

var (
	mu      sync.RWMutex
	current = EN
)

func Current() Lang {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

func Set(lang Lang) {
	mu.Lock()
	defer mu.Unlock()
	current = lang
}

func Parse(raw string) (Lang, bool) {
	n := normalize(raw)
	switch n {
	case EN, PL:
		return n, true
	default:
		return EN, false
	}
}

func DetectFromArgsEnv(args []string, env map[string]string) Lang {
	if l, ok := parseFromArgs(args); ok {
		return l
	}
	for _, key := range []string{"FRISCO_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := strings.TrimSpace(env[key]); v != "" {
			if l, ok := Parse(v); ok {
				return l
			}
		}
	}
	return EN
}

func T(en, pl string) string {
	if Current() == PL && pl != "" {
		return pl
	}
	return en
}

func parseFromArgs(args []string) (Lang, bool) {
	for i := 0; i < len(args); i++ {
		a := strings.TrimSpace(args[i])
		if a == "" {
			continue
		}
		if strings.HasPrefix(a, "--lang=") {
			return Parse(strings.TrimPrefix(a, "--lang="))
		}
		if a == "--lang" && i+1 < len(args) {
			return Parse(args[i+1])
		}
	}
	return EN, false
}

func normalize(raw string) Lang {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return ""
	}
	// Examples: pl_PL.UTF-8, en-US, en_US@calendar=gregorian
	if idx := strings.IndexAny(s, ".@"); idx >= 0 {
		s = s[:idx]
	}
	s = strings.ReplaceAll(s, "_", "-")
	if strings.HasPrefix(s, "pl") {
		return PL
	}
	if strings.HasPrefix(s, "en") {
		return EN
	}
	return ""
}
