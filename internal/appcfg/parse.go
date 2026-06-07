package appcfg

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseText parses apprise TEXT config format into URL entries.
func ParseText(cfg string) []Entry {
	lines := strings.Split(cfg, "\n")
	out := make([]Entry, 0, len(lines))
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") || strings.HasPrefix(ln, ";") {
			continue
		}
		entry := Entry{URL: ln}
		if i := strings.Index(ln, "="); i > 0 {
			entry.Tags = splitTags(strings.TrimSpace(ln[:i]))
			entry.URL = strings.TrimSpace(ln[i+1:])
		}
		if strings.Contains(entry.URL, "://") {
			out = append(out, entry)
		}
	}
	return out
}

// ParseYAML parses apprise YAML config format into URL entries.
func ParseYAML(cfg string) ([]Entry, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(cfg), &root); err != nil {
		return nil, err
	}
	if len(root.Content) == 0 {
		return nil, fmt.Errorf("empty yaml")
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("yaml root must be mapping")
	}

	urls := mappingValue(doc, "urls")
	if urls == nil || urls.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("yaml urls must be sequence")
	}

	out := make([]Entry, 0, len(urls.Content))
	for _, n := range urls.Content {
		switch n.Kind {
		case yaml.ScalarNode:
			if strings.Contains(n.Value, "://") {
				out = append(out, Entry{URL: strings.TrimSpace(n.Value)})
			}
		case yaml.MappingNode:
			if len(n.Content) < 2 {
				continue
			}
			rawURL := strings.TrimSpace(n.Content[0].Value)
			if !strings.Contains(rawURL, "://") {
				continue
			}
			entry := Entry{URL: rawURL}
			if tags := collectYAMLTags(n.Content[1]); len(tags) > 0 {
				entry.Tags = tags
			}
			out = append(out, entry)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no urls in yaml")
	}
	return out, nil
}

// Parse parses by format ("text" or "yaml").
func Parse(format, cfg string) ([]Entry, error) {
	if strings.EqualFold(format, "yaml") {
		return ParseYAML(cfg)
	}
	return ParseText(cfg), nil
}

func collectYAMLTags(n *yaml.Node) []string {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.MappingNode {
		tagNode := mappingValue(n, "tag")
		if tagNode == nil {
			return nil
		}
		switch tagNode.Kind {
		case yaml.ScalarNode:
			return splitTags(tagNode.Value)
		case yaml.SequenceNode:
			tags := make([]string, 0, len(tagNode.Content))
			for _, item := range tagNode.Content {
				if item.Kind == yaml.ScalarNode {
					tags = append(tags, splitTags(item.Value)...)
				}
			}
			return dedupe(tags)
		}
	}
	if n.Kind == yaml.SequenceNode {
		tags := make([]string, 0, len(n.Content))
		for _, item := range n.Content {
			tags = append(tags, collectYAMLTags(item)...)
		}
		return dedupe(tags)
	}
	return nil
}

func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if strings.EqualFold(strings.TrimSpace(m.Content[i].Value), key) {
			return m.Content[i+1]
		}
	}
	return nil
}

func splitTags(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
	return dedupe(parts)
}

func dedupe(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
