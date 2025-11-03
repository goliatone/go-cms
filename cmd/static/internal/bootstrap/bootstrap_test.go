package bootstrap

import "testing"

func TestBuildModuleEnablesGenerator(t *testing.T) {
	resources, err := BuildModule(Options{})
	if err != nil {
		t.Fatalf("build module: %v", err)
	}
	if resources.Module == nil {
		t.Fatal("expected module to be initialised")
	}
	container := resources.Module.Container()
	if container.GeneratorService() == nil {
		t.Fatal("expected generator service to be configured")
	}
}
