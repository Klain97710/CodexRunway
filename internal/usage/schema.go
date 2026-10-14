package usage

// Code generated from the original index schema (version 3); keep byte-for-byte
// in sync so a database written by either implementation stays readable.

// indexSchemaStatements are the statements that create an empty index, in
// the order the original executed them.
var indexSchemaStatements = []string{
	`CREATE TABLE index_metadata (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    schema_version INTEGER NOT NULL,
    parser_version INTEGER NOT NULL,
    aggregation_time_zone TEXT NOT NULL
)`,
	`CREATE TABLE source_files (
    id INTEGER PRIMARY KEY,
    basename TEXT NOT NULL UNIQUE,
    root TEXT NOT NULL,
    path TEXT NOT NULL,
    device INTEGER NOT NULL,
    inode INTEGER NOT NULL,
    birth_ns INTEGER NOT NULL,
    mtime_ns INTEGER NOT NULL,
    ctime_ns INTEGER NOT NULL,
    size INTEGER NOT NULL CHECK (size >= 0),
    complete_offset INTEGER NOT NULL CHECK (complete_offset >= 0 AND complete_offset <= size),
    current_model TEXT NOT NULL,
    current_project TEXT NOT NULL,
    first_hash BLOB NOT NULL CHECK (length(first_hash) = 32),
    first_hash_length INTEGER NOT NULL CHECK (first_hash_length >= 0 AND first_hash_length <= size),
    checkpoint_hash BLOB NOT NULL CHECK (length(checkpoint_hash) = 32),
    checkpoint_hash_length INTEGER NOT NULL CHECK (
        checkpoint_hash_length >= 0 AND checkpoint_hash_length <= complete_offset
    ),
    parser_version INTEGER NOT NULL,
    malformed_lines INTEGER NOT NULL CHECK (malformed_lines >= 0),
    oversized_lines INTEGER NOT NULL CHECK (oversized_lines >= 0),
    full_hash BLOB CHECK (full_hash IS NULL OR length(full_hash) = 32)
)`,
	`CREATE TABLE usage_events (
    file_id INTEGER NOT NULL,
    byte_offset INTEGER NOT NULL CHECK (byte_offset >= 0),
    timestamp REAL NOT NULL,
    day_key TEXT NOT NULL,
    model TEXT NOT NULL,
    project TEXT NOT NULL,
    uncached_input_tokens INTEGER NOT NULL CHECK (uncached_input_tokens >= 0),
    cached_input_tokens INTEGER NOT NULL CHECK (cached_input_tokens >= 0),
    output_tokens INTEGER NOT NULL CHECK (output_tokens >= 0),
    turns INTEGER NOT NULL CHECK (turns >= 0),
    PRIMARY KEY (file_id, byte_offset),
    FOREIGN KEY (file_id) REFERENCES source_files(id) ON DELETE CASCADE
) WITHOUT ROWID`,
	`CREATE TABLE source_hash_cache (
    device INTEGER NOT NULL,
    inode INTEGER NOT NULL,
    birth_ns INTEGER NOT NULL,
    size INTEGER NOT NULL CHECK (size >= 0),
    mtime_ns INTEGER NOT NULL,
    ctime_ns INTEGER NOT NULL,
    full_hash BLOB NOT NULL CHECK (length(full_hash) = 32),
    PRIMARY KEY (device, inode, birth_ns)
) WITHOUT ROWID`,
	`CREATE INDEX usage_events_timestamp_idx ON usage_events(timestamp)`,
}
