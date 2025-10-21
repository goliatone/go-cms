package testsupport

import (
	"encoding/json"
	"os"
)

func LoadFixture(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func LoadGolden(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
