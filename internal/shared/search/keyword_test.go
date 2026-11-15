package search

import (
	"strings"
	"testing"
)

func TestNormalizeKeyword(t *testing.T) {
	blank := "   "
	long := strings.Repeat("长", MaxKeywordLength+1)
	key := "  橄榄油  "

	tests := []struct {
		name    string
		input   *string
		want    *string
		wantErr bool
	}{
		{name: "nil stays nil", input: nil, want: nil},
		{name: "blank becomes nil", input: &blank, want: nil},
		{name: "trims surrounding spaces", input: &key, want: strPtr("橄榄油")},
		{name: "rejects overlong keyword", input: &long, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeKeyword(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %q", *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("expected %q, got %v", *tt.want, got)
			}
		})
	}
}

func strPtr(value string) *string {
	return &value
}
