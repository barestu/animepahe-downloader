package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/barestu/animepahe-downloader/internal/animepahe"
)

// selectEpisodes filters the episode list by a spec:
//
//	"all" / ""   -> every episode
//	"5"          -> episode 5
//	"1-12"       -> episodes 1 through 12
//	"1,3,5-8"    -> mixed list and ranges
//
// Matching is by episode number (the API's `episode` field), not slice index.
func selectEpisodes(eps []animepahe.Episode, spec string) ([]animepahe.Episode, error) {
	spec = strings.TrimSpace(strings.ToLower(spec))
	if spec == "" || spec == "all" {
		return eps, nil
	}

	byNum := make(map[int]animepahe.Episode, len(eps))
	for _, e := range eps {
		byNum[int(e.Episode)] = e
	}

	wanted := make(map[int]bool)
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			a, err1 := strconv.Atoi(strings.TrimSpace(lo))
			b, err2 := strconv.Atoi(strings.TrimSpace(hi))
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("invalid range %q", part)
			}
			if a > b {
				a, b = b, a
			}
			for n := a; n <= b; n++ {
				wanted[n] = true
			}
		} else {
			n, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid episode %q", part)
			}
			wanted[n] = true
		}
	}

	var out []animepahe.Episode
	for _, e := range eps { // preserve sorted order
		if wanted[int(e.Episode)] {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no episodes matched %q", spec)
	}
	return out, nil
}
