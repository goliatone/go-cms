package testsupport

import (
	"encoding/json"
	"os"
)

func LoadFixture(path string) ([]byte, error) {
	return os.ReadFile(path) // #nosec G304 -- tests pass repository fixture paths.
}

func LoadGolden(path string, v any) error {
	data, err := os.ReadFile(path) // #nosec G304 -- tests pass repository golden fixture paths.
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
