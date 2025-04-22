package utils

import (
	"unicode"
	"strings"
)

// Title converts the first letter of each word to uppercase and the rest to lowercase.
func Title(s string) string {
	return strings.Join(titleWords(strings.Fields(s)), " ")
}

func titleWords(words []string) []string {
	for i, word := range words {
		if len(word) == 0 {
			continue
		}
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		for j := 1; j < len(runes); j++ {
			runes[j] = unicode.ToLower(runes[j])
		}
		words[i] = string(runes)
	}
	return words
}
