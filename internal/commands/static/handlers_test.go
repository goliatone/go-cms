package staticcmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/goliatone/go-cms/internal/generator"
	"github.com/google/uuid"
)

func TestBuildSiteHandler_Execute_Build(t *testing.T) {
	cmd := loadBuildFixture(t, "build_basic.json")

	var capturedOpts generator.BuildOptions
	callbackInvoked := false

	svc := &fakeGeneratorService{
		buildFunc: func(ctx context.Context, opts generator.BuildOptions) (*generator.BuildResult, error) {
			capturedOpts = opts
			return &generator.BuildResult{PagesBuilt: 3}, nil
		},
	}

	handler := NewBuildSiteHandler(svc, nil, FeatureGates{GeneratorEnabled: alwaysTrue})

	cmd.ResultCallback = func(env ResultEnvelope) {
		callbackInvoked = true
		if env.Result == nil {
			t.Fatalf("expected build result, got nil")
		}
		if env.Result.PagesBuilt != 3 {
			t.Fatalf("expected PagesBuilt 3, got %d", env.Result.PagesBuilt)
		}
		if env.Metadata["operation"] != "build" {
			t.Fatalf("expected operation build, got %v", env.Metadata["operation"])
		}
	}

	if err := handler.Execute(context.Background(), cmd); err != nil {
		t.Fatalf("execute build: %v", err)
	}

	if capturedOpts.Force != true {
		t.Fatalf("expected Force true, got %v", capturedOpts.Force)
	}
	if capturedOpts.DryRun {
		t.Fatalf("expected DryRun false")
	}
	if len(capturedOpts.PageIDs) != len(cmd.PageIDs) {
		t.Fatalf("expected %d page ids, got %d", len(cmd.PageIDs), len(capturedOpts.PageIDs))
	}
	if !callbackInvoked {
		t.Fatal("expected callback to be invoked")
	}
}

func TestBuildSiteHandler_Execute_AssetsOnly(t *testing.T) {
	cmd := loadBuildFixture(t, "build_assets.json")
	cmd.AssetsOnly = true

	assetsCalled := false
	svc := &fakeGeneratorService{
		buildAssetsFunc: func(ctx context.Context) error {
			assetsCalled = true
			return nil
		},
	}

	handler := NewBuildSiteHandler(svc, nil, FeatureGates{GeneratorEnabled: alwaysTrue})

	callbackInvoked := false
	cmd.ResultCallback = func(env ResultEnvelope) {
		callbackInvoked = true
		if env.Result != nil {
			t.Fatalf("expected nil result for assets build, got %#v", env.Result)
		}
		if env.Metadata["operation"] != "build_assets" {
			t.Fatalf("expected operation build_assets, got %v", env.Metadata["operation"])
		}
	}

	if err := handler.Execute(context.Background(), cmd); err != nil {
		t.Fatalf("execute assets: %v", err)
	}
	if !assetsCalled {
		t.Fatal("expected BuildAssets to be called")
	}
	if !callbackInvoked {
		t.Fatal("expected callback to be invoked")
	}
}

func TestBuildSiteHandler_Execute_PageBuild(t *testing.T) {
	id := uuid.New()
	cmd := BuildSiteCommand{
		PageIDs: []uuid.UUID{id},
		Locales: []string{" en "},
	}

	pageCalled := false
	svc := &fakeGeneratorService{
		buildPageFunc: func(ctx context.Context, pageID uuid.UUID, locale string) error {
			pageCalled = true
			if pageID != id {
				t.Fatalf("expected page id %s, got %s", id, pageID)
			}
			if locale != "en" {
				t.Fatalf("expected locale en, got %s", locale)
			}
			return nil
		},
	}

	handler := NewBuildSiteHandler(svc, nil, FeatureGates{GeneratorEnabled: alwaysTrue})

	callbackInvoked := false
	cmd.ResultCallback = func(env ResultEnvelope) {
		callbackInvoked = true
		if env.Metadata["operation"] != "build_page" {
			t.Fatalf("expected operation build_page, got %v", env.Metadata["operation"])
		}
	}

	if err := handler.Execute(context.Background(), cmd); err != nil {
		t.Fatalf("execute page build: %v", err)
	}
	if !pageCalled {
		t.Fatal("expected BuildPage to be called")
	}
	if !callbackInvoked {
		t.Fatal("expected callback to be invoked")
	}
}

func TestBuildSiteHandler_Execute_GeneratorDisabled(t *testing.T) {
	cmd := BuildSiteCommand{}
	svc := &fakeGeneratorService{}

	handler := NewBuildSiteHandler(svc, nil, FeatureGates{GeneratorEnabled: alwaysFalse})
	err := handler.Execute(context.Background(), cmd)
	if !errors.Is(err, generator.ErrServiceDisabled) {
		t.Fatalf("expected ErrServiceDisabled, got %v", err)
	}
}

func TestDiffSiteHandler_Execute(t *testing.T) {
	cmd := loadDiffFixture(t, "diff_basic.json")

	var capturedOpts generator.BuildOptions
	callbackInvoked := false

	svc := &fakeGeneratorService{
		buildFunc: func(ctx context.Context, opts generator.BuildOptions) (*generator.BuildResult, error) {
			capturedOpts = opts
			return &generator.BuildResult{AssetsBuilt: 2}, nil
		},
	}

	handler := NewDiffSiteHandler(svc, nil, FeatureGates{GeneratorEnabled: alwaysTrue})
	cmd.ResultCallback = func(env ResultEnvelope) {
		callbackInvoked = true
		if env.Metadata["operation"] != "diff" {
			t.Fatalf("expected diff operation, got %v", env.Metadata["operation"])
		}
		if env.Result == nil || env.Result.AssetsBuilt != 2 {
			t.Fatalf("unexpected diff result: %#v", env.Result)
		}
	}

	if err := handler.Execute(context.Background(), cmd); err != nil {
		t.Fatalf("execute diff: %v", err)
	}
	if !capturedOpts.DryRun {
		t.Fatal("expected DryRun to be true for diff")
	}
	if !callbackInvoked {
		t.Fatal("expected callback to be invoked")
	}
}

func TestDiffSiteHandler_Execute_GeneratorDisabled(t *testing.T) {
	handler := NewDiffSiteHandler(&fakeGeneratorService{}, nil, FeatureGates{GeneratorEnabled: alwaysFalse})
	err := handler.Execute(context.Background(), DiffSiteCommand{})
	if !errors.Is(err, generator.ErrServiceDisabled) {
		t.Fatalf("expected ErrServiceDisabled, got %v", err)
	}
}

func TestCleanSiteHandler_Execute(t *testing.T) {
	cleanCalled := false
	svc := &fakeGeneratorService{
		cleanFunc: func(ctx context.Context) error {
			cleanCalled = true
			return nil
		},
	}

	handler := NewCleanSiteHandler(svc, nil, FeatureGates{GeneratorEnabled: alwaysTrue})
	if err := handler.Execute(context.Background(), CleanSiteCommand{}); err != nil {
		t.Fatalf("execute clean: %v", err)
	}
	if !cleanCalled {
		t.Fatal("expected Clean to be called")
	}
}

func TestCleanSiteHandler_Execute_GeneratorDisabled(t *testing.T) {
	handler := NewCleanSiteHandler(&fakeGeneratorService{}, nil, FeatureGates{GeneratorEnabled: alwaysFalse})
	err := handler.Execute(context.Background(), CleanSiteCommand{})
	if !errors.Is(err, generator.ErrServiceDisabled) {
		t.Fatalf("expected ErrServiceDisabled, got %v", err)
	}
}

func TestBuildSiteCommandValidate(t *testing.T) {
	cmd := loadBuildFixture(t, "build_invalid_locale.json")
	if err := cmd.Validate(); err == nil {
		t.Fatal("expected validation error for invalid locales")
	}
}

func loadBuildFixture(t *testing.T, name string) BuildSiteCommand {
	t.Helper()
	var cmd BuildSiteCommand
	loadFixture(t, name, &cmd)
	return cmd
}

func loadDiffFixture(t *testing.T, name string) DiffSiteCommand {
	t.Helper()
	var cmd DiffSiteCommand
	loadFixture(t, name, &cmd)
	return cmd
}

func loadFixture(t *testing.T, name string, target any) {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", name, err)
	}
}

type fakeGeneratorService struct {
	buildFunc        func(context.Context, generator.BuildOptions) (*generator.BuildResult, error)
	buildPageFunc    func(context.Context, uuid.UUID, string) error
	buildAssetsFunc  func(context.Context) error
	buildSitemapFunc func(context.Context) error
	cleanFunc        func(context.Context) error
}

func (f *fakeGeneratorService) Build(ctx context.Context, opts generator.BuildOptions) (*generator.BuildResult, error) {
	if f.buildFunc != nil {
		return f.buildFunc(ctx, opts)
	}
	return nil, nil
}

func (f *fakeGeneratorService) BuildPage(ctx context.Context, pageID uuid.UUID, locale string) error {
	if f.buildPageFunc != nil {
		return f.buildPageFunc(ctx, pageID, locale)
	}
	return nil
}

func (f *fakeGeneratorService) BuildAssets(ctx context.Context) error {
	if f.buildAssetsFunc != nil {
		return f.buildAssetsFunc(ctx)
	}
	return nil
}

func (f *fakeGeneratorService) BuildSitemap(ctx context.Context) error {
	if f.buildSitemapFunc != nil {
		return f.buildSitemapFunc(ctx)
	}
	return generator.ErrNotImplemented
}

func (f *fakeGeneratorService) Clean(ctx context.Context) error {
	if f.cleanFunc != nil {
		return f.cleanFunc(ctx)
	}
	return nil
}

func alwaysTrue() bool  { return true }
func alwaysFalse() bool { return false }
