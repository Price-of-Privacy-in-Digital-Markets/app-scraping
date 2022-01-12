CREATE TABLE IF NOT EXISTS blobs (
    blob_id     INTEGER PRIMARY KEY,
    data        BLOB NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS apps (
    app_id       TEXT PRIMARY KEY NOT NULL CHECK (valid_android_app_id(app_id))
) STRICT, WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS scraped_apps (
    app_id       TEXT NOT NULL REFERENCES apps(app_id),
    scraped_when INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    blob_id      INTEGER REFERENCES blobs(blob_id)
) STRICT;

CREATE INDEX IF NOT EXISTS scraped_apps_app_id ON scraped_apps(app_id);
CREATE INDEX IF NOT EXISTS scraped_apps_blob_id ON scraped_apps(blob_id);

CREATE TABLE IF NOT EXISTS not_found_apps (
    app_id         TEXT NOT NULL REFERENCES apps(app_id),
    not_found_when INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
) STRICT;

CREATE INDEX IF NOT EXISTS not_found_apps_app_id ON not_found_apps(app_id);

CREATE TABLE IF NOT EXISTS prices (
    app_id         TEXT NOT NULL REFERENCES apps(app_id),
    country        TEXT NOT NULL,
    currency       TEXT NOT NULL,
    price          REAL NOT NULL,
    original_price REAL
) STRICT;

CREATE INDEX IF NOT EXISTS prices_app_id ON prices(app_id);
