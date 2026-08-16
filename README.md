# go-wof-client

[![CI](https://github.com/olehmushka/go-wof-client/actions/workflows/ci.yml/badge.svg)](https://github.com/olehmushka/go-wof-client/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/olehmushka/go-wof-client.svg)](https://pkg.go.dev/github.com/olehmushka/go-wof-client)
[![Go Report Card](https://goreportcard.com/badge/github.com/olehmushka/go-wof-client)](https://goreportcard.com/report/github.com/olehmushka/go-wof-client)
[![License](https://img.shields.io/github/license/olehmushka/go-wof-client)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/tag/olehmushka/go-wof-client)](https://github.com/olehmushka/go-wof-client/releases)

A small Go client for [Who's-On-First](https://whosonfirst.org) (WOF) SQLite gazetteer
distributions — per-country `.db.bz2` administrative-places dumps.

## Install

```sh
go get github.com/olehmushka/go-wof-client
```

## Usage

```go
sum, err := wofclient.Download(ctx, govapicore.NewHTTPClient(0), distURL, "/tmp/us.db")

db, err := wofclient.Open("/tmp/us.db")
defer db.Close()

err = db.Placetype(ctx, "country", func(f wofclient.Feature) error {
    // f.Country, f.IsCurrent, f.Body (a WOF GeoJSON Feature — decode it yourself)
    return nil
})
```

## Scope

This client covers downloading + decompressing a distribution and reading its
`spr`/`geojson` tables by placetype. It does not interpret WOF's own property schema
(`wof:id`, `wof:hierarchy`, `wof:name`, ...) into any application's domain model — that
mapping is application-specific and belongs in the caller.

## Notes

Built on [go-govapi-core](https://github.com/olehmushka/go-govapi-core) for the
User-Agent convention, and [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (a
cgo-free `database/sql` driver) to read the distribution. Both the download/decompress
mechanics and the SQL query mirror an already-shipped, already-verified integration
against this same distribution format in
[go-oikumenea](https://github.com/olehmushka/go-oikumenea)'s
`internal/hermenea/fetcher/fetcher.go` (`WOFSQLite`) and
`internal/hermenea/wof/mapper.go` (its D-GeoPlaces / M16 geo-places pipeline) — this
package is that integration's fetch+read layer, extracted and generalized; the
WOF-property-to-application-field mapping stays in the caller.

## License

Apache 2.0 — see [LICENSE](./LICENSE).
