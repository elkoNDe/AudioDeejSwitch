//go:build windows

package util

import (
	"reflect"
	"testing"
)

func TestCloneStrings(t *testing.T) {
	if got := cloneStrings(nil); got != nil {
		t.Errorf("cloneStrings(nil) = %v, want nil", got)
	}

	input := []string{"a", "b"}
	cloned := cloneStrings(input)
	if !reflect.DeepEqual(cloned, input) {
		t.Errorf("cloneStrings() = %v, want %v", cloned, input)
	}

	cloned[0] = "changed"
	if input[0] != "a" {
		t.Error("cloneStrings() returned a slice sharing its backing array")
	}
}

func TestUniqueStrings(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{name: "nil", input: nil, want: nil},
		{name: "empty", input: []string{}, want: nil},
		{name: "deduplicates and skips empty values", input: []string{"a", "", "b", "a", "b"}, want: []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uniqueStrings(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("uniqueStrings(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
