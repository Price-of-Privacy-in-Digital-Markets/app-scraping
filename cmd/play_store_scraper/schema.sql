CREATE TABLE IF NOT EXISTS apps (
    app_id      TEXT PRIMARY KEY NOT NULL CHECK (valid_android_app_id(app_id))
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS scraped_apps (
    scrape_id    INTEGER PRIMARY KEY,
    app_id       TEXT NOT NULL UNIQUE REFERENCES apps(app_id),
    scraped_when INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER)),
    data         BLOB NOT NULL
);

CREATE TABLE IF NOT EXISTS not_found_apps (
    not_found_id INTEGER PRIMARY KEY,
    app_id       TEXT NOT NULL UNIQUE REFERENCES apps(app_id),
    scraped_when INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER))
);

CREATE TABLE IF NOT EXISTS prices (
    scraped_when   INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER)),
    app_id         TEXT NOT NULL REFERENCES apps(app_id),
    country        TEXT NOT NULL CHECK (lower(country) = country),
    currency       TEXT NOT NULL,
    price          REAL NOT NULL CHECK (price >= 0),
    original_price REAL,
    PRIMARY KEY (app_id, country)
);
