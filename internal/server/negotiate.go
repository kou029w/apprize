package server

import (
	"net/http"
	"strconv"
	"strings"
)

// mediaType is a response representation the server can produce. Handlers offer
// the representations they support and negotiate picks one from the request's
// Accept header.
type mediaType int

const (
	mediaJSON mediaType = iota
	mediaText
	mediaHTML
)

// mime returns the bare media type used for Accept matching.
func (m mediaType) mime() string {
	switch m {
	case mediaText:
		return "text/plain"
	case mediaHTML:
		return "text/html"
	default:
		return "application/json"
	}
}

// contentType returns the value written to the Content-Type response header.
func (m mediaType) contentType() string {
	switch m {
	case mediaText:
		return "text/plain; charset=utf-8"
	case mediaHTML:
		return "text/html; charset=utf-8"
	default:
		return "application/json"
	}
}

// negotiate selects the best representation for r from the offered set, which is
// listed in server preference order (best first). When the Accept header is
// absent, is "*/*", or matches none of the offers, the first offer wins — this
// is the endpoint's representative content type per the swagger contract.
func negotiate(r *http.Request, offers ...mediaType) mediaType {
	if len(offers) == 0 {
		return mediaJSON
	}
	accept := strings.TrimSpace(r.Header.Get("Accept"))
	if accept == "" {
		return offers[0]
	}

	ranges := parseAccept(accept)
	best := offers[0]
	bestQ := -1.0
	for _, off := range offers {
		// Strict ">" keeps earlier (more preferred) offers on ties.
		if q := qualityFor(ranges, off.mime()); q > bestQ {
			bestQ = q
			best = off
		}
	}
	if bestQ <= 0 {
		return offers[0]
	}
	return best
}

// mediaRange is a single parsed entry from an Accept header.
type mediaRange struct {
	typ, sub string
	q        float64
}

// parseAccept parses an Accept header into its media ranges, honouring q-values.
func parseAccept(header string) []mediaRange {
	var out []mediaRange
	for _, part := range strings.Split(header, ",") {
		segs := strings.Split(part, ";")
		mt := strings.TrimSpace(segs[0])
		slash := strings.IndexByte(mt, '/')
		if slash < 0 {
			continue
		}
		mr := mediaRange{
			typ: strings.ToLower(mt[:slash]),
			sub: strings.ToLower(mt[slash+1:]),
			q:   1.0,
		}
		for _, p := range segs[1:] {
			if p = strings.TrimSpace(p); strings.HasPrefix(p, "q=") {
				if v, err := strconv.ParseFloat(p[2:], 64); err == nil {
					mr.q = v
				}
			}
		}
		out = append(out, mr)
	}
	return out
}

// qualityFor returns the q-value the ranges assign to mime, picking the most
// specific matching range (exact > type/* > */*). Zero means not acceptable.
func qualityFor(ranges []mediaRange, mime string) float64 {
	slash := strings.IndexByte(mime, '/')
	typ, sub := mime[:slash], mime[slash+1:]
	best := 0.0
	bestSpec := -1
	for _, mr := range ranges {
		spec := matchSpecificity(mr, typ, sub)
		if spec < 0 {
			continue
		}
		if spec > bestSpec || (spec == bestSpec && mr.q > best) {
			bestSpec = spec
			best = mr.q
		}
	}
	return best
}

// matchSpecificity scores how specifically mr matches typ/sub: 2 exact, 1
// type wildcard, 0 full wildcard, -1 no match.
func matchSpecificity(mr mediaRange, typ, sub string) int {
	switch {
	case mr.typ == typ && mr.sub == sub:
		return 2
	case mr.typ == typ && mr.sub == "*":
		return 1
	case mr.typ == "*" && mr.sub == "*":
		return 0
	default:
		return -1
	}
}
