package widgets_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExternalConsumerTypedWidgetCallsCompile(t *testing.T) {
	t.Parallel()

	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	cmd := exec.Command(goBin, "test", "./...")
	cmd.Dir = filepath.Join("testdata", "external_consumer")
	cmd.Env = append(os.Environ(), "GOWORK=off")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("external consumer compile test failed: %v\n%s", err, output)
	}
}
