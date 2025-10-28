module github.com/goliatone/go-cms

go 1.24.0

toolchain go1.24.9

require (
	github.com/go-ozzo/ozzo-validation/v4 v4.3.0
	github.com/goliatone/go-command v0.6.0
	github.com/goliatone/go-errors v0.9.0
	github.com/goliatone/go-repository-bun v0.7.0
	github.com/goliatone/go-repository-cache v0.5.0
	github.com/goliatone/go-urlkit v0.3.0
	github.com/google/uuid v1.6.0
	github.com/mattn/go-sqlite3 v1.14.32
	github.com/uptrace/bun v1.2.15
	github.com/uptrace/bun/dialect/sqlitedialect v1.2.15
)

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/adrg/frontmatter v0.2.0 // indirect
	github.com/alecthomas/kong v1.11.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dlclark/regexp2 v1.11.5 // indirect
	github.com/flosch/pongo2/v6 v6.0.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/soongo/path-to-regexp v1.6.4 // indirect
	github.com/tmthrgd/go-hex v0.0.0-20190904060850-447a3041c3bc // indirect
	github.com/viccon/sturdyc v1.1.5 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/yuin/goldmark v1.6.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
)

replace github.com/goliatone/go-urlkit => ../go-urlkit
