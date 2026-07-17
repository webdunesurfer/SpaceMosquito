package mcp

import "testing"

func TestParseGetPageArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]interface{}
		wantKey string
		wantID  int
		wantErr bool
	}{
		{
			name:    "valid without space_key",
			args:    map[string]interface{}{"confluence_id": float64(1)},
			wantKey: "",
			wantID:  1,
		},
		{
			name:    "valid",
			args:    map[string]interface{}{"space_key": "PROJ", "confluence_id": float64(12345)},
			wantKey: "PROJ",
			wantID:  12345,
		},
		{
			name:    "missing confluence_id",
			args:    map[string]interface{}{"space_key": "PROJ"},
			wantErr: true,
		},
		{
			name:    "invalid confluence_id type",
			args:    map[string]interface{}{"space_key": "PROJ", "confluence_id": "abc"},
			wantErr: true,
		},
		{
			name:    "non-positive confluence_id",
			args:    map[string]interface{}{"space_key": "PROJ", "confluence_id": float64(0)},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, id, err := parseGetPageArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if key != tc.wantKey || id != tc.wantID {
				t.Errorf("got (%q, %d), want (%q, %d)", key, id, tc.wantKey, tc.wantID)
			}
		})
	}
}
