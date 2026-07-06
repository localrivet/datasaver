# Changelog

## [v0.1.3] - 2026-07-06

### Fixed
- pg_dump/pg_restore now receive the full connection string: in URL-only
  setups (DATASAVER_DATABASE_URL) the discrete host/port/user/name fields
  are empty, so every dump silently targeted nothing and exited 1
- pg_dump stderr is included in Dump() errors instead of being discarded

## [v0.1.2] - 2026-07-06

### Fixed
- Upgrade image to postgresql17-client: pg_dump 16 aborts on version mismatch
  against Postgres 17 servers (pg_dump 17 still dumps older servers)

## [v0.1.0] - 2026-01-11

### Added
- Initial release of datasaver
- PostgreSQL and SQLite backup support
- GFS rotation and retention policies
