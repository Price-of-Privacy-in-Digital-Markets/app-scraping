import argparse
import io
import sqlite3

import pyarrow as pa
import pyarrow.parquet as pq
import pyarrow.json

DATA_CATEGORIES_T = pa.list_(pa.struct([
    ("identifier", pa.string()),
    ("data_types", pa.list_(pa.string()))
]))

# country and language are stored as partitions
SCHEMA = pa.schema([
    ("app_id", pa.int64()),
    ("scraped_when", pa.timestamp("ns")),
    ("bundle_id", pa.string()),
    ("title", pa.string()),
    ("description", pa.string()),
    ("icon", pa.string()),
    ("genres", pa.list_(pa.string())),
    ("genre_ids", pa.list_(pa.int64())),
    ("primary_genre", pa.string()),
    ("primary_genre_id", pa.int64()),
    ("content_rating", pa.string()),
    ("content_advisories", pa.list_(pa.string())),
    ("languages", pa.list_(pa.string())),
    ("size", pa.int64()),
    ("required_os_version", pa.string()),
    ("released", pa.timestamp("ns")),
    ("updated", pa.timestamp("ns")),
    ("price", pa.float64()),
    ("currency", pa.string()),
    ("developer_id", pa.int64()),
    ("developer", pa.string()),
    ("developer_url", pa.string()),
    ("developer_website", pa.string()),
    ("score", pa.float64()),
    ("reviews", pa.int64()),
    ("current_version_score", pa.float64()),
    ("current_version_reviews", pa.int64()),
    ("screenshots", pa.list_(pa.string())),
    ("supported_devices", pa.list_(pa.string())),
    pa.field("privacy_nutrition_labels", pa.list_(
        pa.struct([
            ("identifier", pa.string()),
            ("data_categories", DATA_CATEGORIES_T),
            ("purposes", pa.list_(pa.struct([
                ("identifier", pa.string()),
                ("data_categories", DATA_CATEGORIES_T)])))
        ]))
    )
])

def scraped_apps(conn: sqlite3.Connection):
    c = conn.cursor()
    try:
        c.execute(
            """
            SELECT
                json_insert(decompress_brotli(data), '$.scraped_when', datetime(scraped_when, 'unixepoch'))
            FROM scraped_apps
            """
        )
        for json_str, in c:
            yield json_str.encode("utf-8")

    finally:
        c.close()

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Convert App Store scraped data to parquet.")
    parser.add_argument("--sqlite3-extension", type=str, default="sqlite3_tools")
    parser.add_argument("--database", type=str, required=True)
    parser.add_argument("--output", type=str, required=True)

    args = parser.parse_args()

    conn = sqlite3.connect(args.database)
    try:
        conn.enable_load_extension(True)

        try:
            conn.load_extension(args.sqlite3_extension)
        except sqlite3.OperationalError:
            print("Please set the path of the SQLite extension module.")
            exit(1)

        # Read the scraped apps from sqlite to a Python buffer (we need to have enough RAM) and
        # then convert to an arrow table (this isn't that memory intensive as we can share the
        # existing buffer), and then write to parquet.
        # It would be nicer to be able to stream from sqlite without using so much memory but this
        # works well enough for the time being.

        print("Reading scraped apps...")
        buf = io.BytesIO()

        for scraped_app in scraped_apps(conn):
            buf.write(scraped_app)
            buf.write(b"\n")
        
        buf.seek(0)
        
        print("Creating arrow table...")
        tbl = pyarrow.json.read_json(
            pa.py_buffer(buf.getbuffer()),
            parse_options=pyarrow.json.ParseOptions(explicit_schema=SCHEMA, unexpected_field_behavior="ignore")
        )

        print("Writing parquet file...")
        pq.write_table(tbl, args.output, compression="ZSTD", compression_level=19)

    finally:
        conn.close()
