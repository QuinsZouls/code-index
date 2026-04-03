package main

import "testing"

func TestMatchPattern(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/*.go", "src/main.go", true},
		{"*.go", "main.go", true},
		{"src/**/main.go", "src/pkg/app/main.go", true},
		{"src/*.go", "src/pkg/main.go", false},
	}
	for _, tc := range cases {
		if got := matchPattern(tc.pattern, tc.path); got != tc.want {
			t.Fatalf("matchPattern(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestFilepathToSlash(t *testing.T) {
	if got := filepathToSlash("a\\b\\c"); got != "a/b/c" {
		t.Fatalf("filepathToSlash() = %q, want a/b/c", got)
	}
}
