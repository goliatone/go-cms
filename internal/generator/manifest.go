package generator

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	manifestFileName    = ".generator-manifest.json"
	manifestFileVersion = 1
)

// buildManifest stores metadata about the last successful build to support incremental runs.
type buildManifest struct {
	Version     int                        `json:"version"`
	GeneratedAt time.Time                  `json:"generated_at"`
	Pages       map[string]manifestPage    `json:"pages"`
	Assets      map[string]manifestAsset   `json:"assets"`
	Metadata    map[string]json.RawMessage `json:"metadata,omitempty"`
}

type manifestPage struct {
	PageID       string    `json:"page_id"`
	Locale       string    `json:"locale"`
	Route        string    `json:"route"`
	Output       string    `json:"output"`
	Template     string    `json:"template"`
	Hash         string    `json:"hash"`
	Checksum     string    `json:"checksum"`
	LastModified time.Time `json:"last_modified"`
	RenderedAt   time.Time `json:"rendered_at"`
}

type manifestAsset struct {
	Key      string    `json:"key"`
	ThemeID  string    `json:"theme_id"`
	Source   string    `json:"source"`
	Output   string    `json:"output"`
	Checksum string    `json:"checksum"`
	Size     int64     `json:"size"`
	CopiedAt time.Time `json:"copied_at"`
}

func newBuildManifest() *buildManifest {
	return &buildManifest{
		Version:     manifestFileVersion,
		Pages:       map[string]manifestPage{},
		Assets:      map[string]manifestAsset{},
		Metadata:    map[string]json.RawMessage{},
		GeneratedAt: time.Time{},
	}
}

func parseManifest(data []byte) (*buildManifest, error) {
	if len(data) == 0 {
		return newBuildManifest(), nil
	}
	var manifest buildManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("generator: parse manifest: %w", err)
	}
	if manifest.Pages == nil {
		manifest.Pages = map[string]manifestPage{}
	}
	if manifest.Assets == nil {
		manifest.Assets = map[string]manifestAsset{}
	}
	if manifest.Metadata == nil {
		manifest.Metadata = map[string]json.RawMessage{}
	}
	if manifest.Version == 0 {
		manifest.Version = manifestFileVersion
	}
	return &manifest, nil
}

func (m *buildManifest) marshal() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	cloned := *m
	if cloned.Version == 0 {
		cloned.Version = manifestFileVersion
	}
	if cloned.Pages == nil {
		cloned.Pages = map[string]manifestPage{}
	}
	if cloned.Assets == nil {
		cloned.Assets = map[string]manifestAsset{}
	}
	if cloned.Metadata == nil {
		cloned.Metadata = map[string]json.RawMessage{}
	}
	// Stable ordering for deterministic output.
	type orderedManifest struct {
		Version     int                        `json:"version"`
		GeneratedAt time.Time                  `json:"generated_at"`
		Pages       []manifestPage             `json:"pages"`
		Assets      []manifestAsset            `json:"assets"`
		Metadata    map[string]json.RawMessage `json:"metadata,omitempty"`
	}
	ordered := orderedManifest{
		Version:     cloned.Version,
		GeneratedAt: cloned.GeneratedAt,
		Metadata:    cloned.Metadata,
	}
	if len(cloned.Pages) > 0 {
		ordered.Pages = make([]manifestPage, 0, len(cloned.Pages))
		for _, entry := range cloned.Pages {
			ordered.Pages = append(ordered.Pages, entry)
		}
		sort.Slice(ordered.Pages, func(i, j int) bool {
			if ordered.Pages[i].PageID == ordered.Pages[j].PageID {
				return ordered.Pages[i].Locale < ordered.Pages[j].Locale
			}
			return ordered.Pages[i].PageID < ordered.Pages[j].PageID
		})
	}
	if len(cloned.Assets) > 0 {
		ordered.Assets = make([]manifestAsset, 0, len(cloned.Assets))
		for _, entry := range cloned.Assets {
			ordered.Assets = append(ordered.Assets, entry)
		}
		sort.Slice(ordered.Assets, func(i, j int) bool {
			return ordered.Assets[i].Key < ordered.Assets[j].Key
		})
	}
	return json.MarshalIndent(ordered, "", "  ")
}

func (m *buildManifest) pageKey(pageID uuid.UUID, locale string) string {
	return strings.ToLower(pageID.String()) + "::" + strings.ToLower(strings.TrimSpace(locale))
}

func (m *buildManifest) assetKey(themeID uuid.UUID, source string) string {
	return strings.ToLower(themeID.String()) + "::" + strings.TrimSpace(source)
}

func (m *buildManifest) lookupPage(pageID uuid.UUID, locale string) (manifestPage, bool) {
	if m == nil || len(m.Pages) == 0 {
		return manifestPage{}, false
	}
	entry, ok := m.Pages[m.pageKey(pageID, locale)]
	return entry, ok
}

func (m *buildManifest) setPage(entry manifestPage) {
	if m == nil {
		return
	}
	if m.Pages == nil {
		m.Pages = map[string]manifestPage{}
	}
	key := strings.ToLower(strings.TrimSpace(entry.PageID)) + "::" + strings.ToLower(strings.TrimSpace(entry.Locale))
	m.Pages[key] = entry
}

func (m *buildManifest) shouldSkipPage(pageID uuid.UUID, locale, hash, output string) bool {
	entry, ok := m.lookupPage(pageID, locale)
	if !ok {
		return false
	}
	if entry.Hash != hash {
		return false
	}
	if strings.TrimSpace(entry.Output) != strings.TrimSpace(output) {
		return false
	}
	return true
}

func (m *buildManifest) lookupAsset(themeID uuid.UUID, source string) (manifestAsset, bool) {
	if m == nil || len(m.Assets) == 0 {
		return manifestAsset{}, false
	}
	entry, ok := m.Assets[m.assetKey(themeID, source)]
	return entry, ok
}

func (m *buildManifest) setAsset(entry manifestAsset) {
	if m == nil {
		return
	}
	if m.Assets == nil {
		m.Assets = map[string]manifestAsset{}
	}
	key := strings.ToLower(entry.Key)
	if key == "" {
		key = strings.ToLower(entry.ThemeID) + "::" + strings.TrimSpace(entry.Source)
		entry.Key = key
	}
	m.Assets[key] = entry
}

func (m *buildManifest) shouldSkipAsset(themeID uuid.UUID, source, checksum, output string) bool {
	entry, ok := m.lookupAsset(themeID, source)
	if !ok {
		return false
	}
	if entry.Checksum != checksum {
		return false
	}
	if strings.TrimSpace(entry.Output) != strings.TrimSpace(output) {
		return false
	}
	return true
}

func (m *buildManifest) manifestPath(baseDir string) string {
	base := strings.Trim(strings.TrimSpace(baseDir), "/")
	return joinOutputPath(base, manifestFileName)
}

func (m *buildManifest) prunePages(keys map[string]struct{}) {
	if len(keys) == 0 || len(m.Pages) == 0 {
		return
	}
	for key := range m.Pages {
		if _, ok := keys[key]; !ok {
			delete(m.Pages, key)
		}
	}
}

func (m *buildManifest) pruneAssets(keys map[string]struct{}) {
	if len(keys) == 0 || len(m.Assets) == 0 {
		return
	}
	for key := range m.Assets {
		if _, ok := keys[key]; !ok {
			delete(m.Assets, key)
		}
	}
}

func manifestDir(pathValue string) string {
	dir := strings.TrimSpace(path.Dir(strings.TrimSpace(pathValue)))
	if dir == "." {
		return ""
	}
	return dir
}
