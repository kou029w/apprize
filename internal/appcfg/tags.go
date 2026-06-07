package appcfg

import "strings"

// ParseTagExpr parses "a b, c" into groups: [["a","b"],["c"]].
func ParseTagExpr(expr string) [][]string {
	expr = strings.TrimSpace(expr)
	if expr == "" || strings.EqualFold(expr, "all") {
		return [][]string{{"all"}}
	}
	ors := strings.FieldsFunc(expr, func(r rune) bool { return r == ',' || r == '|' })
	out := make([][]string, 0, len(ors))
	for _, group := range ors {
		ands := strings.Fields(group)
		if len(ands) == 0 {
			continue
		}
		out = append(out, ands)
	}
	if len(out) == 0 {
		return [][]string{{"all"}}
	}
	return out
}

// Match checks if entryTags satisfy the parsed expression groups.
func Match(groups [][]string, entryTags []string) bool {
	if len(groups) == 0 {
		return true
	}
	if len(groups) == 1 && len(groups[0]) == 1 && strings.EqualFold(groups[0][0], "all") {
		return true
	}
	tagSet := map[string]struct{}{}
	for _, tag := range entryTags {
		tagSet[strings.ToLower(strings.TrimSpace(tag))] = struct{}{}
	}
	for _, group := range groups {
		ok := true
		for _, term := range group {
			if _, hit := tagSet[strings.ToLower(strings.TrimSpace(term))]; !hit {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
