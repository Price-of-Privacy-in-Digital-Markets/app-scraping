import argparse
import datetime
import io
import sqlite3

import pyarrow as pa
import pyarrow.parquet as pq
import pyarrow.json

HISTOGRAM_T = pa.struct([
    ("1", pa.int64()),
    ("2", pa.int64()),
    ("3", pa.int64()),
    ("4", pa.int64()),
    ("5", pa.int64()),
])

PERMISSION_T = pa.struct([
    ("group", pa.string()),
    ("permission", pa.string()),
])

DATA_TYPE_T = pa.struct([
    ("name", pa.string()),
    ("optional", pa.bool_()),
    ("purposes", pa.string())
])

DATA_CATEGORY_T = pa.struct([
    ("category", pa.string()),
    ("data_types", pa.list_(DATA_TYPE_T))
])

DATA_SAFETY_T = pa.struct([
    ("collection", pa.list_(DATA_CATEGORY_T)),
    ("sharing", pa.list_(DATA_CATEGORY_T)),
    ("security_practices", pa.list_(pa.string()))
])

SCHEMA = pa.schema([
    pa.field("app_id", pa.string(), nullable=False),
    pa.field("scraped_when", pa.timestamp("s")),
    pa.field("title", pa.string()),
    pa.field("description", pa.string()),
    pa.field("summary", pa.string()),
    pa.field("installs", pa.string()),
    pa.field("min_installs", pa.int64()),
    pa.field("max_installs", pa.int64()),
    pa.field("score", pa.float64()),
    pa.field("ratings", pa.int64()),
    pa.field("reviews", pa.int64()),
    pa.field("histogram", HISTOGRAM_T),
    pa.field("price", pa.float64()),
    pa.field("currency", pa.string()),
    pa.field("sale_end_time", pa.timestamp("s")),
    pa.field("original_price", pa.float64()),
    pa.field("sale_text", pa.string()),
    pa.field("available", pa.bool_()),
    pa.field("in_app_purchases", pa.bool_()),
    pa.field("in_app_purchases_range", pa.string()),
    pa.field("min_api", pa.int32()),
    pa.field("target_api", pa.int32()),
    pa.field("min_android_version", pa.string()),
    pa.field("developer", pa.string()),
    pa.field("developer_id", pa.string()),
    pa.field("developer_email", pa.string()),
    pa.field("developer_website", pa.string()),
    pa.field("developer_address", pa.string()),
    pa.field("privacy_policy", pa.string()),
    pa.field("genre_id", pa.string()),
    pa.field("additional_genre_ids", pa.list_(pa.string())),
    pa.field("teacher_approved_age", pa.string()),
    pa.field("icon", pa.string()),
    pa.field("header_image", pa.string()),
    pa.field("screenshots", pa.list_(pa.string())),
    pa.field("video", pa.string()),
    pa.field("video_image", pa.string()),
    pa.field("content_rating", pa.string()),
    pa.field("content_rating_description", pa.string()),
    pa.field("ad_supported", pa.bool_()),
    pa.field("released", pa.timestamp("s")),
    pa.field("updated", pa.timestamp("s")),
    pa.field("version", pa.string()),
    pa.field("recent_changes", pa.string()),
    pa.field("recent_changes_time", pa.timestamp("s")),
    pa.field("similar", pa.list_(pa.string())),
    pa.field("permissions", pa.list_(PERMISSION_T)),
    pa.field("data_safety", DATA_SAFETY_T)
])

PRICE_SCHEMA = pa.schema([
    pa.field("scraped_when", pa.timestamp("s"), nullable=False),
    pa.field("app_id", pa.string(), nullable=False),
    pa.field("country", pa.string(), nullable=False),
    pa.field("currency", pa.string(), nullable=False),
    pa.field("price", pa.float64(), nullable=False),
    pa.field("original_price", pa.float64(), nullable=True)
])

def scraped_apps(conn: sqlite3.Connection):
    c = conn.cursor()
    try:
        # For parquet, similar is just a list of app IDs so we have to extract each ID from the
        # similar object
        c.execute(
            """
            SELECT
                json_set(
                    sa.json,
                    '$.scraped_when', datetime(sa.scraped_when, 'unixepoch'),
                    '$.similar', json_group_array(json_extract(similar.value, '$.app_id'))
                )
            FROM
                (SELECT scrape_id, scraped_when, decompress_brotli(data) AS json FROM scraped_apps) AS sa,
                json_each(sa.json, '$.similar') As similar
            GROUP BY
                scrape_id
            """
        )
        for json_str, in c:
            yield json_str.encode("utf-8")

    finally:
        c.close()

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Convert Play Store scraped data to parquet.")
    parser.add_argument("--sqlite3-extension", type=str, default="sqlite3_tools")
    parser.add_argument("--database", type=str, required=True)
    parser.add_argument("--output-scraped", help="Path of scraped apps Parquet file", type=str, required=True)
    parser.add_argument("--output-prices", help="Path of prices Parquet file", type=str, required=True)

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
        pq.write_table(tbl, args.output_scraped, compression="ZSTD", compression_level=19)

        del(buf)
    
        # Now extract price information

        print("Reading prices...")
        rows = conn.execute(
            """
            SELECT
                datetime(scraped_when, 'unixepoch'),
                app_id,
                country,
                currency,
                price,
                original_price
            FROM
                prices
            """
        ).fetchall()

        print("Creating arrow table...")
        scraped_when, app_id, country, currency, price, original_price = zip(*rows)
        tbl = pa.Table.from_pydict(
            {
                "scraped_when": [datetime.datetime.fromisoformat(dt) for dt in scraped_when],
                "app_id": app_id,
                "country": country,
                "currency": currency,
                "price": price,
                "original_price": original_price
            },
            schema=PRICE_SCHEMA
        )

        print("Writing parquet file...")
        pq.write_table(tbl, args.output_prices, compression="ZSTD", compression_level=19)

    finally:
        conn.close()
