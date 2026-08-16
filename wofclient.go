// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package wofclient is a small client for Who's-On-First (WOF) SQLite gazetteer
// distributions (https://whosonfirst.org) — per-country `.db.bz2` administrative-places
// dumps.
//
// It covers two things: downloading + decompressing a distribution to a local SQLite
// file (Download), and reading its `spr`/`geojson` tables by placetype (DB.Placetype).
// It does not interpret WOF's own property schema (wof:id, wof:hierarchy, wof:name,
// ...) into any application's domain model — that mapping is application-specific and
// belongs in the caller, not in a generic client to this distribution format.
//
// Both the download/decompress mechanics and the SQL query mirror an already-shipped,
// already-verified integration against this same distribution format:
// https://github.com/olehmushka/go-oikumenea/blob/main/internal/hermenea/fetcher/fetcher.go
// (its `WOFSQLite` streaming fetcher) and
// https://github.com/olehmushka/go-oikumenea/blob/main/internal/hermenea/wof/mapper.go
// (its D-GeoPlaces / M16 geo-places pipeline, which reads the exact query this package
// runs) — this package is that integration's fetch+read layer, extracted and
// generalized; the WOF-property-to-application-field mapping stays in the caller.
package wofclient

import (
	"compress/bzip2"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"

	govapicore "github.com/olehmushka/go-govapi-core"
	_ "modernc.org/sqlite" // registers the cgo-free "sqlite" database/sql driver
)

// UserAgentEnv lets an operator override the outbound User-Agent with their own
// contact.
const UserAgentEnv = "WOF_CLIENT_USER_AGENT"

// DefaultUserAgent identifies this client when UserAgentEnv is unset.
const DefaultUserAgent = "go-wof-client/0.1 (+https://github.com/olehmushka/go-wof-client)"

// Download streams a WOF distribution's `.db.bz2` from url, bzip2-decompresses it to
// dest (a local file path), and returns the sha256 checksum of the decompressed bytes
// (hex-encoded). The decompressed bytes are streamed straight to disk — a WOF
// distribution can be gigabytes of geometry, so this never buffers the whole thing in
// memory. client's own timeout (or the caller's ctx deadline, if client has none)
// governs how long this may run; a planet-scale distribution can take a while, so a
// caller downloading one of those should use an unbounded client
// (govapicore.NewHTTPClient(0)) bounded by ctx instead.
func Download(ctx context.Context, client *http.Client, url, dest string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("wofclient: build request for %s: %w", url, err)
	}
	req.Header.Set("User-Agent", govapicore.ResolveUserAgent(UserAgentEnv, DefaultUserAgent))

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("wofclient: GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", &govapicore.ErrUnexpectedStatus{URL: url, Status: resp.Status, Body: body}
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("wofclient: create %s: %w", dest, err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), bzip2.NewReader(resp.Body)); err != nil {
		return "", fmt.Errorf("wofclient: decompress %s: %w", url, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DB wraps an opened WOF SQLite distribution for read access.
type DB struct {
	sqlDB *sql.DB
}

// Open opens a WOF SQLite distribution at path (as produced by Download, or any WOF
// `.db` file already decompressed by other means).
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("wofclient: open %s: %w", path, err)
	}
	return &DB{sqlDB: sqlDB}, nil
}

// Close closes the underlying database handle.
func (d *DB) Close() error { return d.sqlDB.Close() }

// Feature is one WOF `spr` row joined to its canonical GeoJSON body — the raw shape;
// Body is a WOF GeoJSON Feature (properties + geometry) the caller decodes itself.
type Feature struct {
	Country   string
	IsCurrent int
	Body      []byte
}

// Placetype streams every current-geometry feature for one WOF placetype (e.g.
// "country", "region", "county", "locality"), ordered by the distribution's internal
// id (parent-first within a single placetype's insert order — WOF's own convention),
// calling yield once per row so a caller can page without loading a whole placetype
// into memory. yield returning an error stops iteration and that error is returned.
func (d *DB) Placetype(ctx context.Context, placetype string, yield func(Feature) error) error {
	rows, err := d.sqlDB.QueryContext(ctx, `
		SELECT s.country, s.is_current, g.body
		FROM spr s
		JOIN geojson g ON g.id = s.id
		WHERE s.placetype = ? AND g.is_alt = 0
		ORDER BY s.id`, placetype)
	if err != nil {
		return fmt.Errorf("wofclient: query placetype %s: %w", placetype, err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			country   sql.NullString
			isCurrent sql.NullInt64
			body      []byte
		)
		if err := rows.Scan(&country, &isCurrent, &body); err != nil {
			return fmt.Errorf("wofclient: scan placetype %s row: %w", placetype, err)
		}
		if err := yield(Feature{Country: country.String, IsCurrent: int(isCurrent.Int64), Body: body}); err != nil {
			return err
		}
	}
	return rows.Err()
}
