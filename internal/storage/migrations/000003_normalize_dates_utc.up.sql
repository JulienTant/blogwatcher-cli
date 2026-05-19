-- Normalize existing timestamps to UTC RFC3339-ish format.
-- Pre-existing rows may carry timezone offsets (e.g. "+09:00") because feeds
-- supply timestamps in their local zone. The date filter compares strings
-- lexicographically, which only behaves correctly for a uniform zone, so we
-- rewrite all stored timestamps to UTC. strftime('%Y-%m-%dT%H:%M:%fZ', x)
-- accepts ISO8601 with offset or "YYYY-MM-DD HH:MM:SS" and emits UTC.

UPDATE articles
SET published_date = strftime('%Y-%m-%dT%H:%M:%fZ', published_date)
WHERE published_date IS NOT NULL;

UPDATE articles
SET discovered_date = strftime('%Y-%m-%dT%H:%M:%fZ', discovered_date)
WHERE discovered_date IS NOT NULL;

UPDATE blogs
SET last_scanned = strftime('%Y-%m-%dT%H:%M:%fZ', last_scanned)
WHERE last_scanned IS NOT NULL;
