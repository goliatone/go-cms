package cms

import (
	"errors"
	"testing"
)

func TestCanonicalMenuCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "spaces", in: "  \t ", want: ""},
		{name: "uppercase", in: "Admin", want: "admin"},
		{name: "trim", in: " admin ", want: "admin"},
		{name: "dot_to_underscore", in: "admin.main", want: "admin_main"},
		{name: "dot_uppercase", in: "Admin.Main", want: "admin_main"},
		{name: "slash_to_dash", in: "admin/main", want: "admin-main"},
		{name: "leading_trailing_punct", in: "---admin---", want: "admin"},
		{name: "dot_only", in: ".", want: ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := CanonicalMenuCode(tc.in); got != tc.want {
				t.Fatalf("expected %q got %q", tc.want, got)
			}
		})
	}
}

func TestCanonicalMenuItemPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		menuCode string
		raw      string
		want     string
		wantErr  error
	}{
		{
			name:     "dot_path_prefixed",
			menuCode: "Admin",
			raw:      "Admin.Content.Pages",
			want:     "admin.content.pages",
		},
		{
			name:     "slash_path_prefixed",
			menuCode: "Admin",
			raw:      "Admin/Content/Pages",
			want:     "admin.content.pages",
		},
		{
			name:     "relative_slash_path",
			menuCode: "Admin",
			raw:      "content/pages",
			want:     "admin.content.pages",
		},
		{
			name:     "relative_single_segment",
			menuCode: "Admin",
			raw:      "Pages",
			want:     "admin.pages",
		},
		{
			name:     "empty_menu_code",
			menuCode: "   ",
			raw:      "admin.pages",
			wantErr:  ErrMenuCodeRequired,
		},
		{
			name:     "empty_path",
			menuCode: "admin",
			raw:      "   ",
			wantErr:  ErrMenuItemPathRequired,
		},
		{
			name:     "unrecoverable_path",
			menuCode: "admin",
			raw:      "...",
			wantErr:  ErrMenuItemPathInvalid,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := CanonicalMenuItemPath(tc.menuCode, tc.raw)
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v got nil (path=%q)", tc.wantErr, got)
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q got %q", tc.want, got)
			}
			if _, err := ParseMenuItemPathForMenu(tc.menuCode, got); err != nil {
				t.Fatalf("expected output to parse for menu: %v", err)
			}
		})
	}
}

func TestDeriveMenuItemPaths(t *testing.T) {
	t.Parallel()

	t.Run("without_parent", func(t *testing.T) {
		t.Parallel()

		got, err := DeriveMenuItemPaths("Admin", "Admin/Content/Pages", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Path != "admin.content.pages" {
			t.Fatalf("expected path admin.content.pages got %q", got.Path)
		}
		if got.ParentPath != "" {
			t.Fatalf("expected empty parent path got %q", got.ParentPath)
		}
	})

	t.Run("with_parent_single_segment_id_becomes_child", func(t *testing.T) {
		t.Parallel()

		got, err := DeriveMenuItemPaths("Admin", "Pages", "Content", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Path != "admin.content.pages" {
			t.Fatalf("expected path admin.content.pages got %q", got.Path)
		}
		if got.ParentPath != "admin.content" {
			t.Fatalf("expected parent admin.content got %q", got.ParentPath)
		}
	})

	t.Run("missing_id_uses_fallback_label", func(t *testing.T) {
		t.Parallel()

		got, err := DeriveMenuItemPaths("Admin", "", "", "Reports & Analytics")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Path != "admin.reports-analytics" {
			t.Fatalf("expected path admin.reports-analytics got %q", got.Path)
		}
		if got.ParentPath != "" {
			t.Fatalf("expected empty parent path got %q", got.ParentPath)
		}
	})

	t.Run("missing_id_empty_fallback_uses_item", func(t *testing.T) {
		t.Parallel()

		got, err := DeriveMenuItemPaths("Admin", "", "admin", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Path != "admin.item" {
			t.Fatalf("expected path admin.item got %q", got.Path)
		}
		if got.ParentPath != "admin" {
			t.Fatalf("expected parent admin got %q", got.ParentPath)
		}
	})

	t.Run("invalid_parent_errors", func(t *testing.T) {
		t.Parallel()

		_, err := DeriveMenuItemPaths("Admin", "Pages", "!!!", "")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !errors.Is(err, ErrMenuItemPathInvalid) {
			t.Fatalf("expected ErrMenuItemPathInvalid got %v", err)
		}
	})
}

func TestSeedPositionPtrForType(t *testing.T) {
	t.Parallel()

	if SeedPositionPtrForType("item", -1) != nil {
		t.Fatalf("expected nil for pos<0")
	}
	if got := SeedPositionPtrForType("item", 1); got == nil || *got != 1 {
		t.Fatalf("expected &1 for pos>0")
	}
	if SeedPositionPtrForType("item", 0) != nil {
		t.Fatalf("expected nil for item pos==0")
	}
	if got := SeedPositionPtrForType(" group ", 0); got == nil || *got != 0 {
		t.Fatalf("expected &0 for group pos==0")
	}
	if got := SeedPositionPtrForType("separator", 0); got == nil || *got != 0 {
		t.Fatalf("expected &0 for separator pos==0")
	}
	if SeedPositionPtrForType("unknown", 0) != nil {
		t.Fatalf("expected nil for unknown type pos==0")
	}
}
