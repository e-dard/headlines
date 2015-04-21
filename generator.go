package headlines

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"strings"
	"time"
)

// delim is the delimter used to separate tokens.
const delim = " "

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Chain represents a Markov Chain.
//
// When a Chain is built, it maintains the state space and state
// transistions in a map of indexes, where each index refers to an index
// of a corresponding slice of tokens.
//
// The Chain also contains a slice of indexes for the first token found
// in every sentence in the corpus. These are used to begin generated
// phrases.
//
// To map indexes to the actual tokens Indexes are used.
//
type Chain struct {
	// TODO(e-dard): There are way better ways to store the transitions and
	// representation. One day get around to improving these.

	// Stores the state space—every key is a token (prefix) and the
	// value associated with that key are indexes of all of the possible
	// tokens (suffixes) that can follow.
	//
	// chain also implicitly stores the state transition structure. Each
	// suffix index is duplicated in the slice according to how many
	// times it is found adjacent to the prefix in the corpus. Therefore
	// transition from a prefix to the next state can be achieved by
	// uniformly drawing a value in [0, n) where n is the size of the
	// index slice.
	chain map[string][]int32

	// tokensIdx maps an index from the chain to a token.
	tokensIdx index

	// Indexes to the first tokens encountered in each example in the
	// corpus. Used as the first token (starting state) when generating
	// new examples. As with the values of chain, the indexes are
	// duplicated according to their frequency as the first token in the
	// corpus.
	sp []int32

	// startingPrefixIdx maps an index from sp to a token.
	startingPrefixIdx index

	// how large to make each prefix, in terms of number of tokens.
	prefixLength int
}

// NewChain creates a new Chain.
//
// A Chain uses a prefix length l to determine how many tokens (words)
// to consider when deciding on the next token in a generated phrase.
func NewChain(l int) *Chain {
	return &Chain{
		chain:        map[string][]int32{},
		prefixLength: l,
	}
}

// Build consumes from a reader and first builds an index of all tokens
// read. Then is re-reads from the reader using the built index to
// construct a mapping between prefixes and suffixes.
//
// The underlying data stream should contain phrases separated by
// \n.
func (c *Chain) Build(r io.Reader) error {
	// Multiplex r over multiple buffers, so that we can have a full
	// reader left over for building the chain.
	b1, b2 := &bytes.Buffer{}, &bytes.Buffer{}
	// All reads from tr will be written to b1 and b2.
	tr := io.TeeReader(r, io.MultiWriter(b1, b2))

	// Build tokens Index.
	if err := c.buildTokenIndex(tr); err != nil {
		return err
	}

	// Build starting prefix Index using b1.
	if err := c.buildPrefixIndex(b1); err != nil {
		return err
	}

	// Build chain mapping from buffer.
	return c.buildChain(b2)
}

// buildTokenIndex builds an index of all tokens, which is used by the
// chain to reduce the space needed to map prefixes to suffixes.
// Once this index is built, a Chain only needs to map string prefixes
// to int32 values (the suffixes), which are simply indexes to the
// Index built by this function.
func (c *Chain) buildTokenIndex(r io.Reader) error {
	processTokens := func(tokens []string) {
		for _, token := range tokens {
			c.tokensIdx.Add(token)
		}
	}
	return processStream(r, processTokens)
}

// buildPrefixIndex builds an index of all starting prefixes, that is,
// all prefixes that begin individual phrases.
//
// Once this index is built, a Chain only needs to represent all
// starting prefixes as a slice of indexes to the Index built by this
// function.
func (c *Chain) buildPrefixIndex(r io.Reader) error {
	processStartingPrefixes := func(tokens []string) {
		if len(tokens) >= c.prefixLength {
			// join the first prefixLength tokens in the tokens
			// together, and add them to the starting prefix index.
			prefix := strings.Join(tokens[:c.prefixLength], delim)
			c.startingPrefixIdx.Add(prefix)
		}
	}
	return processStream(r, processStartingPrefixes)
}

// buildChain builds up the markov chain mapping by examining each
// phrase read from the provided reader, tokenizing it, and storing
// mappings between prefixes (one or more tokens) and the following
// suffix.
func (c *Chain) buildChain(r io.Reader) error {
	// store each prefix mapped to the following token.
	processPhrase := func(tokens []string) {
		for i := 0; i < len(tokens)-c.prefixLength; i++ {
			prefix := strings.Join(tokens[i:i+c.prefixLength], delim)
			suffix := tokens[i+c.prefixLength]

			// add prefix and suffix mapping
			suffixI := c.tokensIdx.Find(suffix)
			if suffixI == -1 {
				// should not be possible
				panic("can't find token in Index")
			}
			c.chain[prefix] = append(c.chain[prefix], suffixI)

			// is this the start of a line?
			if i == 0 {
				startingPrefixI := c.startingPrefixIdx.Find(prefix)
				if startingPrefixI == -1 {
					//should not be possible
					panic("can't find starting prefix in Index")
				}
				c.sp = append(c.sp, startingPrefixI)
			}
		}
	}
	return processStream(r, processPhrase)
}

// MustGenerate panics if Generate returns an error.
func (c *Chain) MustGenerate(length int) string {
	s, err := c.Generate(length)
	if err != nil {
		panic(err)
	}
	return s
}

// Generate uses the Markov chain to generate a phrase with a maximum
// length of l.
func (c *Chain) Generate(l int) (string, error) {
	// Pick a starting prefix with a probability proportionate to
	// the frequency by which a phrase starts with it.
	// This works due to c.sp containing duplicate starting prefixes,
	// so they're sampled according to their frequency.
	i := rand.Intn(len(c.sp))
	prefix := c.startingPrefixIdx.Get(int(c.sp[i]))
	sentence := strings.Split(prefix, delim)

	if len(sentence) < c.prefixLength {
		return "", fmt.Errorf("sentence must begin with at least %d tokens\n", c.prefixLength)
	}

	for len(sentence) < l {
		prefix := sentence[len(sentence)-c.prefixLength:]
		// All suffix indexes associated with prefix.
		suffixesI := c.chain[strings.Join(prefix, delim)]
		if len(suffixesI) == 0 {
			break
		}

		// Pick a suffix. Suffixes are duplicated, so the probability
		// of selection is proportionate to their frequency.
		i := rand.Intn(len(suffixesI))
		suffix := c.tokensIdx.Get(int(suffixesI[i]))
		sentence = append(sentence, suffix)
	}
	return strings.Join(sentence, delim), nil
}

// processStream consumes from a reader, reading the input line by line.
//
// Each line is tokenized, but splitting on delim, and each slice of
// tokens are passed into processTokens.
//
// processStream does not return an error when it encounters io.EOF.
func processStream(r io.Reader, processTokens func([]string)) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes(byte('\n'))
		if err != nil && err != io.EOF {
			return err
		}

		str := strings.TrimSpace(string(line))
		tokens := strings.Split(str, delim)

		processTokens(tokens)
		if err == io.EOF {
			break
		}
	}
	return nil
}

// index stores a set of tokens.
//
// The set is represented as a sorted array, so while elements can be
// accessed in-place, their position is not stable as elements are added
// to the index.
type index struct {
	tokens []string
}

// Add adds token to the Index, if token does not already exist.
func (idx *index) Add(token string) {
	i := sort.SearchStrings(idx.tokens, token)
	if i == len(idx.tokens) {
		// not in Index
		idx.tokens = append(idx.tokens, token)
		return
	}

	// check if token does not exist in index
	if idx.tokens[i] != token {
		// make room for new token
		idx.tokens = append(idx.tokens, "")
		// move right hand side up
		copy(idx.tokens[i+1:], idx.tokens[i:])
		// insert new token into correct place
		idx.tokens[i] = token
	}
}

// Get returns whatever token is at position i.
func (idx *index) Get(i int) string {
	return idx.tokens[i]
}

// Find searches in O(log n) time for token, and returns the position it
// is located at.
//
// If token is not present in the Index, then Find returns -1.
func (idx *index) Find(token string) int32 {
	i := sort.SearchStrings(idx.tokens, token)
	if i == len(idx.tokens) || idx.tokens[i] != token {
		// not in index
		return int32(-1)
	}
	return int32(i)
}

// String implements fmt.Stringer.
func (idx index) String() string {
	var out string
	for _, t := range idx.tokens {
		out = fmt.Sprintf("%v\n%v", out, t)
	}
	return out + "\n"
}
