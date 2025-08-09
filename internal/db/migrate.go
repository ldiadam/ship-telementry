package db

import (
	"database/sql"
	"io"
	"os"
)

func Migrate(db *sql.DB) error {
	schemaFile, err := os.Open("schema/schema.sql")
	if err != nil {
		return err
	}
	defer schemaFile.Close()

	schema, err := io.ReadAll(schemaFile)
	if err != nil {
		return err
	}

	_, err = db.Exec(string(schema))
	return err
}
