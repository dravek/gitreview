package cli

import "testing"

func TestParseSupportsBaseFlagAnywhere(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want Config
	}{
		{
			name: "path only",
			args: []string{"repo"},
			want: Config{Path: "repo"},
		},
		{
			name: "base only",
			args: []string{"--base", "main"},
			want: Config{Path: ".", BaseBranch: "main"},
		},
		{
			name: "path before base",
			args: []string{"repo", "--base", "main"},
			want: Config{Path: "repo", BaseBranch: "main"},
		},
		{
			name: "base before path",
			args: []string{"--base", "main", "repo"},
			want: Config{Path: "repo", BaseBranch: "main"},
		},
		{
			name: "version only",
			args: []string{"--version"},
			want: Config{Path: ".", ShowVersion: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.args)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if got != tt.want {
				t.Fatalf("Parse() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseRejectsUnknownFlags(t *testing.T) {
	if _, err := Parse([]string{"--wat"}); err == nil {
		t.Fatal("Parse() expected error for unknown flag")
	}
}
