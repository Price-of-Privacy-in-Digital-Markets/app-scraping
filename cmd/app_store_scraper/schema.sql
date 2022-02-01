CREATE TABLE IF NOT EXISTS apps (
    app_id      INT PRIMARY KEY NOT NULL
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS scraped_apps (
    scrape_id    INTEGER PRIMARY KEY,
    app_id       INT NOT NULL UNIQUE REFERENCES apps(app_id),
    scraped_when INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER)),
    data         BLOB NOT NULL
);

CREATE TABLE IF NOT EXISTS not_found_apps (
    not_found_id INTEGER PRIMARY KEY,
    app_id       INT NOT NULL UNIQUE REFERENCES apps(app_id),
    scraped_when INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER))
);

CREATE TABLE IF NOT EXISTS spider_progress (
    genre        INTEGER NOT NULL,
    letter       TEXT NOT NULL,
    page_reached INTEGER,
    PRIMARY KEY (genre, letter)
);

INSERT INTO spider_progress
WITH
    -- From https://github.com/facundoolano/app-store-scraper/blob/master/lib/constants.js#L19
    genre (genre) AS (
        VALUES  (6018),
                (6000),
                (6022),
                (6017),
                (6016),
                (6015),
                (6023),
                (6014),
                (7001),
                (7002),
                (7003),
                (7004),
                (7005),
                (7006),
                (7007),
                (7008),
                (7009),
                (7011),
                (7012),
                (7013),
                (7014),
                (7015),
                (7016),
                (7017),
                (7018),
                (7019),
                (6013),
                (6012),
                (6021),
                (13007),
                (13006),
                (13008),
                (13009),
                (13010),
                (13011),
                (13012),
                (13013),
                (13014),
                (13015),
                (13002),
                (13017),
                (13018),
                (13003),
                (13019),
                (13020),
                (13021),
                (13001),
                (13004),
                (13023),
                (13024),
                (13025),
                (13026),
                (13027),
                (13005),
                (13028),
                (13029),
                (13030),
                (6020),
                (6011),
                (6010),
                (6009),
                (6008),
                (6007),
                (6006),
                (6024),
                (6005),
                (6004),
                (6003),
                (6002),
                (6001)
    ),
    letter (letter) AS (
        VALUES  ("A"),
                ("B"),
                ("C"),
                ("D"),
                ("E"),
                ("F"),
                ("G"),
                ("H"),
                ("I"),
                ("J"),
                ("K"),
                ("L"),
                ("M"),
                ("N"),
                ("O"),
                ("P"),
                ("Q"),
                ("R"),
                ("S"),
                ("T"),
                ("U"),
                ("V"),
                ("W"),
                ("X"),
                ("Y"),
                ("Z"),
                ("*")
    )
SELECT genre, letter, 1 AS page_reached FROM genre, letter WHERE true
ON CONFLICT DO NOTHING;
