package precortex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// SimpleTokenizer provides basic BERT-style tokenization
type SimpleTokenizer struct {
	vocab    map[string]int
	invVocab map[int]string
	unkToken string
	unkID    int
	clsToken string
	clsID    int
	sepToken string
	sepID    int
	padToken string
	padID    int
}

// NewSimpleTokenizer loads a tokenizer from vocab files
func NewSimpleTokenizer(dir string) (*SimpleTokenizer, error) {
	vocabPath := filepath.Join(dir, "vocab.json")
	vocabBytes, err := os.ReadFile(vocabPath)
	if err != nil {
		return nil, err
	}

	var vocab map[string]int
	if err := json.Unmarshal(vocabBytes, &vocab); err != nil {
		return nil, err
	}

	// Build inverse vocab
	invVocab := make(map[int]string, len(vocab))
	for k, v := range vocab {
		invVocab[v] = k
	}

	t := &SimpleTokenizer{
		vocab:    vocab,
		invVocab: invVocab,
		unkToken: "[UNK]",
		clsToken: "[CLS]",
		sepToken: "[SEP]",
		padToken: "[PAD]",
	}

	// Get special token IDs
	if id, ok := vocab[t.unkToken]; ok {
		t.unkID = id
	}
	if id, ok := vocab[t.clsToken]; ok {
		t.clsID = id
	}
	if id, ok := vocab[t.sepToken]; ok {
		t.sepID = id
	}
	if id, ok := vocab[t.padToken]; ok {
		t.padID = id
	}

	return t, nil
}

// NewFallbackTokenizer creates a simple fallback tokenizer
func NewFallbackTokenizer() *SimpleTokenizer {
	return &SimpleTokenizer{
		vocab:    make(map[string]int),
		invVocab: make(map[int]string),
		unkToken: "[UNK]",
		unkID:    1,
		clsToken: "[CLS]",
		clsID:    101,
		sepToken: "[SEP]",
		sepID:    102,
		padToken: "[PAD]",
		padID:    0,
	}
}

// Tokenize converts text to token IDs
func (t *SimpleTokenizer) Tokenize(text string) []int {
	// Lowercase and clean
	text = strings.ToLower(strings.TrimSpace(text))

	// Split into words
	words := t.splitIntoWords(text)

	// Convert to token IDs
	tokens := []int{t.clsID} // Start with [CLS]

	for _, word := range words {
		// Try full word first
		if id, ok := t.vocab[word]; ok {
			tokens = append(tokens, id)
		} else {
			// Try wordpiece tokenization
			subTokens := t.tokenizeWord(word)
			tokens = append(tokens, subTokens...)
		}
	}

	tokens = append(tokens, t.sepID) // End with [SEP]
	return tokens
}

// splitIntoWords splits text into words
func (t *SimpleTokenizer) splitIntoWords(text string) []string {
	var words []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			if unicode.IsPunct(r) {
				words = append(words, string(r))
			}
		} else {
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// tokenizeWord applies wordpiece tokenization to a single word
func (t *SimpleTokenizer) tokenizeWord(word string) []int {
	if len(word) == 0 {
		return nil
	}

	// If word is in vocab, return it
	if id, ok := t.vocab[word]; ok {
		return []int{id}
	}

	// Try character by character (simplified wordpiece)
	tokens := []int{}
	start := 0

	for start < len(word) {
		end := len(word)
		found := false

		for end > start {
			substr := word[start:end]
			if start > 0 {
				substr = "##" + substr
			}

			if id, ok := t.vocab[substr]; ok {
				tokens = append(tokens, id)
				found = true
				break
			}
			end--
		}

		if !found {
			// Unknown character, use [UNK]
			tokens = append(tokens, t.unkID)
			start++
		} else {
			start = end
		}
	}

	return tokens
}
