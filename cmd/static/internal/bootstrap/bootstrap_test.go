package bootstrap

import "testing"

func TestBuildModuleEnablesGenerator(t *testing.T) {
	module, err := BuildModule(Options{})
	if err != nil {
		t.Fatalf("build module: %v", err)
	}
	container := module.Container()
	if container.GeneratorService() == nil {
		t.Fatal("expected generator service to be configured")
	}
}
