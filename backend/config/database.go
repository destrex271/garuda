package config

import (
	"context"

	"github.com/jackc/pgx/v5"
)

var Conn *pgx.Conn
var DATABASE_URI string = "postgres://postgres:postgres@localhost:5432/api_inventory"
var err error

func Connect() error {
	Conn, err = pgx.Connect(context.Background(), DATABASE_URI)
	return err
}

func CloseConnection() error {
	err = Conn.Close(context.Background())
	return err
}
