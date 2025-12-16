package cms

import "testing"

func TestParseMenuItemPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		path       string
		wantMenu   string
		wantParent string
		wantKey    string
		wantErr    bool
	}{
		{
			name:       "nested",
			path:       "admin.content.pages",
			wantMenu:   "admin",
			wantParent: "admin.content",
			wantKey:    "pages",
		},
		{
			name:       "root_item",
			path:       "admin.tenants",
			wantMenu:   "admin",
			wantParent: "admin",
			wantKey:    "tenants",
		},
		{
			name:    "missing_dot",
			path:    "admin",
			wantErr: true,
		},
		{
			name:       "leading_dot",
			path:       ".admin.content",
			wantMenu:   "admin",
			wantParent: "admin",
			wantKey:    "content",
		},
		{
			name:       "trailing_dot",
			path:       "admin.content.",
			wantMenu:   "admin",
			wantParent: "admin",
			wantKey:    "content",
		},
		{
			name:       "double_dot",
			path:       "admin..content",
			wantMenu:   "admin",
			wantParent: "admin",
			wantKey:    "content",
		},
		{
			name:       "invalid_segment_space",
			path:       "admin.con tent.pages",
			wantMenu:   "admin",
			wantParent: "admin.con-tent",
			wantKey:    "pages",
		},
		{
			name:       "invalid_segment_symbol",
			path:       "admin.content.$pages",
			wantMenu:   "admin",
			wantParent: "admin.content",
			wantKey:    "pages",
		},
		{
			name:    "empty",
			path:    "   ",
			wantErr: true,
		},
		{
			name:       "slash_path",
			path:       "Admin/Content/Pages",
			wantMenu:   "admin",
			wantParent: "admin.content",
			wantKey:    "pages",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := ParseMenuItemPath(tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (%+v)", parsed)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if parsed.MenuCode != tc.wantMenu {
				t.Fatalf("MenuCode: expected %q got %q", tc.wantMenu, parsed.MenuCode)
			}
			if parsed.ParentPath != tc.wantParent {
				t.Fatalf("ParentPath: expected %q got %q", tc.wantParent, parsed.ParentPath)
			}
			if parsed.Key != tc.wantKey {
				t.Fatalf("Key: expected %q got %q", tc.wantKey, parsed.Key)
			}
		})
	}
}

func TestParseMenuItemPathForMenu(t *testing.T) {
	t.Parallel()

	if _, err := ParseMenuItemPathForMenu("admin", "site.home"); err == nil {
		t.Fatalf("expected mismatch error")
	}
	if _, err := ParseMenuItemPathForMenu("", "admin.home"); err == nil {
		t.Fatalf("expected menu code required error")
	}
	if parsed, err := ParseMenuItemPathForMenu("admin", "admin.home"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if parsed.ParentPath != "admin" {
		t.Fatalf("expected parent to be admin, got %q", parsed.ParentPath)
	}
	if parsed, err := ParseMenuItemPathForMenu("Admin", "ADMIN/Pages"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if parsed.Path != "admin.pages" {
		t.Fatalf("expected canonical path to be admin.pages, got %q", parsed.Path)
	}
}
