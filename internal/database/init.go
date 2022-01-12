package database

import (
	"database/sql"

	sqlite "github.com/mattn/go-sqlite3"
)

func init() {
	sql.Register("sqlite3_custom", &sqlite.SQLiteDriver{
		ConnectHook: func(conn *sqlite.SQLiteConn) error {
			if err := conn.RegisterFunc("valid_android_app_id", validAppId, true); err != nil {
				return err
			}
			return nil
		},
	})
}
