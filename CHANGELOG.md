# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-08-16

### Added

- Initial extraction from go-oikumenea's `internal/hermenea/fetcher` (`WOFSQLite`) and
  `internal/hermenea/wof` (`GeoPlacesMapper`'s SQL read): `Download` (fetch +
  bzip2-decompress a WOF `.db.bz2` distribution to disk) and `DB.Placetype` (stream a
  placetype's current-geometry features), independent of any downstream project's
  WOF-property-to-domain-model mapping.

[Unreleased]: https://github.com/olehmushka/go-wof-client/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/olehmushka/go-wof-client/releases/tag/v0.1.0
