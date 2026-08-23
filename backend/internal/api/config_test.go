package api

import "testing"

func TestParsePositiveInt(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "positive", value: "120", want: 120},
		{name: "zero", value: "0", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "not number", value: "abc", wantErr: true},
		{name: "empty", value: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePositiveInt(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parsePositiveInt(%q) returned nil error, want error", tt.value)
				}
				return
			}

			if err != nil {
				t.Fatalf("parsePositiveInt(%q) returned error: %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("parsePositiveInt(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}
