import argparse
from pathlib import Path

import pyarrow as pa
import pyarrow.parquet as pq
import pyarrow.json

histogram_t = pa.struct([
    ("1", pa.int64()),
    ("2", pa.int64()),
    ("3", pa.int64()),
    ("4", pa.int64()),
    ("5", pa.int64()),
])

permission_t = pa.struct([
    ("group", pa.string()),
    ("permission", pa.string()),
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
    pa.field("histogram", histogram_t),
    pa.field("price", pa.float64()),
    pa.field("currency", pa.string()),
    pa.field("sale", pa.bool_()),
    pa.field("sale_time", pa.timestamp("s")),
    pa.field("original_price", pa.float64()),
    pa.field("sale_text", pa.string()),
    pa.field("available", pa.bool_()),
    pa.field("in_app_purchases", pa.bool_()),
    pa.field("in_app_purchases_range", pa.string()),
    pa.field("size", pa.string()),
    pa.field("android_version", pa.string()),
    pa.field("developer", pa.string()),
    pa.field("developer_id", pa.int64()),
    pa.field("developer_email", pa.string()),
    pa.field("developer_website", pa.string()),
    pa.field("developer_address", pa.string()),
    pa.field("privacy_policy", pa.string()),
    pa.field("genre_id", pa.string()),
    pa.field("family_genre_id", pa.string()),
    pa.field("icon", pa.string()),
    pa.field("header_image", pa.string()),
    pa.field("screenshots", pa.list_(pa.string())),
    pa.field("video", pa.string()),
    pa.field("video_image", pa.string()),
    pa.field("content_rating", pa.string()),
    pa.field("content_rating_description", pa.string()),
    pa.field("ad_supported", pa.bool_()),
    pa.field("updated", pa.timestamp("s")),
    pa.field("version", pa.string()),
    pa.field("recent_changes", pa.string()),
    pa.field("comments", pa.list_(pa.string())),
    pa.field("editors_choice", pa.bool_()),
    pa.field("similar", pa.list_(pa.string())),
    pa.field("permissions", pa.list_(permission_t))
])

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Convert Play Store JSON to parquet.")
    parser.add_argument("--input", type=str, required=True)
    parser.add_argument("--output", type=str, required=True)

    args = parser.parse_args()

    tbl = pyarrow.json.read_json(
        args.input,
        parse_options=pa.json.ParseOptions(explicit_schema=SCHEMA, unexpected_field_behavior="ignore")
    )
    
    pq.write_table(tbl, args.output, compression="ZSTD", compression_level=19)
