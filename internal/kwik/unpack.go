// Package kwik resolves Kwik (kwik.cx) links into a downloadable resource.
//
// Kwik obfuscates its pages with the Dean-Edwards "p,a,c,k,e,d" JavaScript
// packer. Unpack reverses it deterministically (no JS engine needed) so we can
// pull the .m3u8 URL or the direct-download form token out of the page.
package kwik

import (
	"fmt"
	"regexp"
	"strings"
)

// argsRe captures the packer payload, radix, count and the '|'-joined symbol
// table from the tail of `}('payload', radix, count, 'a|b|c'.split('|'), 0, {})`.
var argsRe = regexp.MustCompile(`(?s)\}\s*\(\s*'(.*?)'\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*'(.*?)'\.split\('\|'\)`)

var wordRe = regexp.MustCompile(`\b\w+\b`)

// Unpack reverses a Dean-Edwards packed script, returning the original source.
func Unpack(packed string) (string, error) {
	m := argsRe.FindStringSubmatch(packed)
	if m == nil {
		return "", fmt.Errorf("kwik: no packed payload found")
	}
	payload := m[1]
	radix := atoi(m[2])
	count := atoi(m[3])
	symtab := strings.Split(m[4], "|")

	if radix < 2 {
		radix = 36
	}
	if len(symtab) != count {
		// not fatal: replacement is bounded by symtab length below
	}

	ub := newUnbaser(radix)
	out := wordRe.ReplaceAllStringFunc(payload, func(word string) string {
		idx := ub.unbase(word)
		if idx >= 0 && idx < len(symtab) && symtab[idx] != "" {
			return symtab[idx]
		}
		return word
	})
	return out, nil
}

// unbaser converts packer tokens (base-N words) back to their integer index.
type unbaser struct {
	base     int
	alphabet string
	dict     map[rune]int
}

const fullAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func newUnbaser(base int) *unbaser {
	u := &unbaser{base: base}
	if base > 36 {
		if base > len(fullAlphabet) {
			base = len(fullAlphabet)
			u.base = base
		}
		u.alphabet = fullAlphabet[:base]
		u.dict = make(map[rune]int, base)
		for i, c := range u.alphabet {
			u.dict[c] = i
		}
	}
	return u
}

func (u *unbaser) unbase(s string) int {
	if u.base <= 36 {
		// base-36 and below: standard digit/letter parsing.
		n := 0
		for _, c := range strings.ToLower(s) {
			d := digitValue(c)
			if d < 0 || d >= u.base {
				return -1
			}
			n = n*u.base + d
		}
		return n
	}
	n := 0
	for _, c := range s {
		d, ok := u.dict[c]
		if !ok {
			return -1
		}
		n = n*u.base + d
	}
	return n
}

func digitValue(c rune) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	default:
		return -1
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
