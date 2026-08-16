// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package wofclient

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// bzip2Compress shells out to the system `bzip2` binary to compress data — Go's stdlib
// only provides a bzip2 *reader*, no writer, and this test needs a real bz2 stream to
// exercise Download's bzip2.NewReader path faithfully rather than faking it.
func bzip2Compress(t *testing.T, data []byte) []byte {
	t.Helper()
	if _, err := exec.LookPath("bzip2"); err != nil {
		t.Skip("bzip2 binary not available in this environment")
	}
	cmd := exec.Command("bzip2", "-c")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bzip2 -c: %v", err)
	}
	return out
}

func TestDownloadDecompressesAndChecksums(t *testing.T) {
	original := []byte(`{"this is a fake WOF sqlite file body, repeated for compressibility": "` +
		string(make([]byte, 200)) + `"}`)
	compressed := bzip2Compress(t, original)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(compressed)
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "test.db")
	client := &http.Client{}
	sum, err := Download(context.Background(), client, srv.URL, dest)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if sum == "" {
		t.Fatal("checksum is empty")
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read decompressed file: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("decompressed content mismatch: got %d bytes, want %d bytes", len(got), len(original))
	}
}

func TestDownloadUpstreamFailurePassesThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := Download(context.Background(), &http.Client{}, srv.URL, filepath.Join(t.TempDir(), "test.db"))
	if err == nil {
		t.Fatal("a real 404 from the upstream should be a real error")
	}
}

// buildTestWOFDB creates a minimal SQLite file at path with the real WOF spr/geojson
// schema shape this package's query depends on, and inserts a few rows.
func buildTestWOFDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE spr (id INTEGER PRIMARY KEY, placetype TEXT, country TEXT, is_current INTEGER)`,
		// id is NOT unique alone: WOF carries one row per (id, is_alt) — a feature can have
		// multiple alternate geometries sharing its id, distinguished by is_alt.
		`CREATE TABLE geojson (id INTEGER, is_alt INTEGER, body TEXT, PRIMARY KEY (id, is_alt))`,
		`INSERT INTO spr (id, placetype, country, is_current) VALUES (1, 'country', 'US', 1)`,
		`INSERT INTO geojson (id, is_alt, body) VALUES (1, 0, '{"properties":{"wof:id":1,"wof:name":"United States"}}')`,
		`INSERT INTO spr (id, placetype, country, is_current) VALUES (2, 'country', 'CA', 0)`,
		`INSERT INTO geojson (id, is_alt, body) VALUES (2, 0, '{"properties":{"wof:id":2,"wof:name":"Canada"}}')`,
		// An alt geometry row for the same feature — must be excluded (is_alt = 0 filter).
		`INSERT INTO geojson (id, is_alt, body) VALUES (2, 1, '{"properties":{"wof:id":2,"wof:name":"Canada (alt)"}}')`,
		// A different placetype — must not appear in a "country" query.
		`INSERT INTO spr (id, placetype, country, is_current) VALUES (3, 'region', 'US', 1)`,
		`INSERT INTO geojson (id, is_alt, body) VALUES (3, 0, '{"properties":{"wof:id":3,"wof:name":"California"}}')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
}

func TestPlacetypeStreamsMatchingFeaturesOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wof-test.db")
	buildTestWOFDB(t, path)

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	var got []Feature
	err = db.Placetype(context.Background(), "country", func(f Feature) error {
		got = append(got, f)
		return nil
	})
	if err != nil {
		t.Fatalf("Placetype: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d features, want 2 (country rows only, alt geometry excluded)", len(got))
	}
	if got[0].Country != "US" || got[0].IsCurrent != 1 {
		t.Errorf("got[0] = %+v, want Country=US IsCurrent=1", got[0])
	}
	if got[1].Country != "CA" || got[1].IsCurrent != 0 {
		t.Errorf("got[1] = %+v, want Country=CA IsCurrent=0", got[1])
	}
	for _, f := range got {
		if bytes.Contains(f.Body, []byte("alt")) {
			t.Errorf("got an alt-geometry body: %s", f.Body)
		}
	}
}

func TestPlacetypeYieldErrorStopsIteration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wof-test.db")
	buildTestWOFDB(t, path)

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	calls := 0
	wantErr := context.Canceled
	err = db.Placetype(context.Background(), "country", func(f Feature) error {
		calls++
		return wantErr
	})
	if err != wantErr {
		t.Fatalf("Placetype err = %v, want the yield error passed through", err)
	}
	if calls != 1 {
		t.Fatalf("yield called %d times, want exactly 1 (iteration should stop on first error)", calls)
	}
}
