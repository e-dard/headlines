package headlines

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"reflect"

	"testing"
)

func Test_Index_Add(t *testing.T) {
	i := index{}

	examples := []struct {
		token    string
		expected []string
	}{
		{token: "foo", expected: []string{"foo"}},
		{token: "foo", expected: []string{"foo"}},
		{token: "hello", expected: []string{"foo", "hello"}},
		{token: "abc", expected: []string{"abc", "foo", "hello"}},
		{token: "boo", expected: []string{"abc", "boo", "foo", "hello"}},
		{token: "boo", expected: []string{"abc", "boo", "foo", "hello"}},
	}

	for _, ex := range examples {
		i.Add(ex.token)
		if !reflect.DeepEqual(i.tokens, ex.expected) {
			t.Fatalf("expected %v, got %v\n", ex.expected, i.tokens)
		}
	}
}

func Test_Index_Get(t *testing.T) {
	i := index{
		tokens: []string{"abc", "boo", "foo", "hello"},
	}

	examples := []struct {
		i        int
		expected string
	}{
		{i: 0, expected: "abc"},
		{i: 0, expected: "abc"},
		{i: 2, expected: "foo"},
	}

	for _, ex := range examples {
		if v := i.Get(ex.i); v != ex.expected {
			t.Fatalf("expected %v, got %v\n", ex.expected, v)
		}
	}
}

func Test_Index_Find(t *testing.T) {
	i := index{
		tokens: []string{"abc", "boo", "foo", "hello"},
	}

	examples := []struct {
		token    string
		expected int32
	}{
		{token: "foo", expected: 2},
		{token: "hello", expected: 3},
		{token: "HELLO", expected: -1},
		{token: "fo", expected: -1},
		{token: "abc", expected: 0},
	}

	for _, ex := range examples {
		if pos := i.Find(ex.token); pos != ex.expected {
			t.Fatalf("expected %v, got %v\n", ex.expected, pos)
		}
	}
}

func Test_BuildChain(t *testing.T) {
	c := NewChain(2)
	data := `the quick brown fox, jumps over the lazy dog.
foo bar brown fox, hello.
foo bar zoo.`

	if err := c.Build(bytes.NewBufferString(data)); err != nil {
		t.Fatal(err)
	}

	expectedTokens := []string{
		"bar", "brown", "dog.",
		"foo", "fox,", "hello.",
		"jumps", "lazy", "over",
		"quick", "the", "zoo.",
	}
	if !reflect.DeepEqual(expectedTokens, c.tokensIdx.tokens) {
		t.Fatalf("expected %v, got %v\n", expectedTokens, c.tokensIdx.tokens)
	}

	expectedPrefixes := []string{"foo bar", "the quick"}
	if !reflect.DeepEqual(expectedPrefixes, c.startingPrefixIdx.tokens) {
		t.Fatalf("expected %v, got %v\n", expectedPrefixes, c.startingPrefixIdx.tokens)
	}

	expectedsp := []int32{1, 0, 0}
	if !reflect.DeepEqual(expectedsp, c.sp) {
		t.Fatalf("expected %v, got %v\n", expectedsp, c.sp)
	}

	expectedC := map[string][]int32{
		"the quick": []int32{1}, "quick brown": []int32{4},
		"brown fox,": []int32{6, 5}, "fox, jumps": []int32{8},
		"jumps over": []int32{10}, "over the": []int32{7},
		"the lazy": []int32{2}, "foo bar": []int32{1, 11},
		"bar brown": []int32{4},
	}
	if !reflect.DeepEqual(expectedC, c.chain) {
		t.Fatalf("expected %v, got %v\n", expectedC, c.chain)
	}

}

func benchmark_Build(prefix int, b *testing.B) {
	log.SetOutput(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		c := NewChain(prefix)
		// open corpus
		f, err := os.Open("bench.txt")
		if err != nil {
			panic(err)
		}
		b.StartTimer()
		if err := c.Build(f); err != nil {
			b.Fatal(err)
		}
		f.Close()
	}
}

func Benchmark_Build_Prefix_2(b *testing.B) {
	benchmark_Build(2, b)
}

func Benchmark_Build_Prefix_3(b *testing.B) {
	benchmark_Build(3, b)
}
