-- Normalize existing timestamps to UTC, fixed-width RFC3339 (no fractional).
--
-- Pre-existing rows may carry timezone offsets (e.g. "+09:00") because feeds
-- supply timestamps in their local zone. The date filter compares strings
-- lexicographically, which only behaves correctly across a uniform zone and a
-- uniform width, so we rewrite all stored timestamps to UTC second-precision.
--
-- COALESCE preserves the original value if strftime() returns NULL (which it
-- does for unparseable inputs) so we surface bad data instead of silently
-- destroying it.

UPDATE articles
SET published_date = COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ', published_date), published_date)
WHERE published_date IS NOT NULL;

UPDATE articles
SET discovered_date = COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ', discovered_date), discovered_date)
WHERE discovered_date IS NOT NULL;

UPDATE blogs
SET last_scanned = COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ', last_scanned), last_scanned)
WHERE last_scanned IS NOT NULL;
