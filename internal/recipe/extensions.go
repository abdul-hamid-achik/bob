package recipe

import (
	"regexp"
	"sort"
	"strings"
)

// MatchExtensionPoints returns the most-specific metadata extension patterns
// matching a canonical repository-relative path. A forbidden path never
// matches, and a placeholder consumes characters within one path segment
// only. Specificity prevents a focused CLI convention from also being labeled
// as a generic domain-package extension.
func MatchExtensionPoints(metadata Metadata, path string) []ExtensionPointDefinition {
	type scoredMatch struct {
		definition  ExtensionPointDefinition
		specificity int
	}
	scored := []scoredMatch{}
	bestSpecificity := -1
	for _, extension := range metadata.ExtensionPoints {
		forbidden := false
		for _, candidate := range extension.ForbiddenPaths {
			if candidate == path {
				forbidden = true
				break
			}
		}
		if forbidden {
			continue
		}
		extensionSpecificity := -1
		for _, pattern := range extension.CreatePatterns {
			if matchPathTemplate(pattern, path) {
				if specificity := pathTemplateSpecificity(pattern); specificity > extensionSpecificity {
					extensionSpecificity = specificity
				}
			}
		}
		if extensionSpecificity >= 0 {
			scored = append(scored, scoredMatch{definition: extension, specificity: extensionSpecificity})
			if extensionSpecificity > bestSpecificity {
				bestSpecificity = extensionSpecificity
			}
		}
	}
	matches := []ExtensionPointDefinition{}
	for _, match := range scored {
		if match.specificity == bestSpecificity {
			matches = append(matches, match.definition)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	return matches
}

func pathTemplateSpecificity(pattern string) int {
	specificity := 0
	inPlaceholder := false
	for _, char := range pattern {
		switch char {
		case '<':
			inPlaceholder = true
		case '>':
			inPlaceholder = false
		default:
			if !inPlaceholder {
				specificity++
			}
		}
	}
	return specificity
}

func matchPathTemplate(pattern, path string) bool {
	patternSegments := strings.Split(pattern, "/")
	pathSegments := strings.Split(path, "/")
	if len(patternSegments) != len(pathSegments) {
		return false
	}
	for i := range patternSegments {
		expression := "^"
		remaining := patternSegments[i]
		for {
			start := strings.IndexByte(remaining, '<')
			if start < 0 {
				expression += regexp.QuoteMeta(remaining)
				break
			}
			end := strings.IndexByte(remaining[start:], '>')
			if end < 0 {
				return false
			}
			end += start
			expression += regexp.QuoteMeta(remaining[:start]) + ".+"
			remaining = remaining[end+1:]
		}
		expression += "$"
		matched, err := regexp.MatchString(expression, pathSegments[i])
		if err != nil || !matched {
			return false
		}
	}
	return true
}
