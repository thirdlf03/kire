package output

import (
	"strings"
	"testing"
)

func TestValidateSubPath(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		sub     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "simple filename",
			base:    "/tmp/out",
			sub:     "file.md",
			wantErr: false,
		},
		{
			name:    "nested path",
			base:    "/tmp/out",
			sub:     "sub/file.md",
			wantErr: false,
		},
		{
			name:    "traversal with dotdot",
			base:    "/tmp/out",
			sub:     "../etc/passwd",
			wantErr: true,
			errMsg:  "path traversal detected",
		},
		{
			name:    "traversal with middle dotdot",
			base:    "/tmp/out",
			sub:     "sub/../../etc/passwd",
			wantErr: true,
			errMsg:  "path traversal detected",
		},
		{
			name:    "absolute path stays inside (Go filepath.Join behavior)",
			base:    "/tmp/out",
			sub:     "/etc/passwd",
			wantErr: false,
		},
		{
			name:    "current directory",
			base:    "/tmp/out",
			sub:     ".",
			wantErr: false,
		},
		{
			name:    "dotdot only",
			base:    "/tmp/out",
			sub:     "..",
			wantErr: true,
			errMsg:  "path traversal detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateSubPath(tt.base, tt.sub)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
