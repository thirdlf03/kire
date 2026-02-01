package tokenizer_test

import (
	"testing"

	"github.com/thirdlf03/kire/internal/tokenizer"
)

func TestLocalEstimator_Empty(t *testing.T) {
	est := tokenizer.NewLocalEstimator()
	n, err := est.Estimate("")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 tokens for empty string, got %d", n)
	}
}

func TestLocalEstimator_ASCII(t *testing.T) {
	est := tokenizer.NewLocalEstimator()
	// "hello world" = 11 bytes, ceil(11/4) = 3
	n, err := est.Estimate("hello world")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("expected 3 tokens for 'hello world', got %d", n)
	}
}

func TestLocalEstimator_NonASCII(t *testing.T) {
	est := tokenizer.NewLocalEstimator()
	// "日本語" = 9 bytes (3 chars × 3 bytes), ceil(9/3) = 3
	n, err := est.Estimate("日本語")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("expected 3 tokens for '日本語', got %d", n)
	}
}

func TestLocalEstimator_Mixed(t *testing.T) {
	est := tokenizer.NewLocalEstimator()
	// "Hello日本語" = 5 ASCII bytes + 9 non-ASCII bytes
	// ceil(5/4 + 9/3) = ceil(1.25 + 3) = ceil(4.25) = 5
	n, err := est.Estimate("Hello日本語")
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Errorf("expected 5 tokens for 'Hello日本語', got %d", n)
	}
}

func TestLocalEstimator_LongText(t *testing.T) {
	est := tokenizer.NewLocalEstimator()
	// 100 ASCII chars → ceil(100/4) = 25
	text := ""
	for i := 0; i < 100; i++ {
		text += "a"
	}
	n, err := est.Estimate(text)
	if err != nil {
		t.Fatal(err)
	}
	if n != 25 {
		t.Errorf("expected 25 tokens for 100 ASCII chars, got %d", n)
	}
}
