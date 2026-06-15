package main

import (
	"database/sql"
	"flag"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	dsn := flag.String("dsn", "postgres://regime:regime@localhost:5432/regime?sslmode=disable", "postgres DSN")
	dir := flag.String("dir", "migrations", "migrations directory")
	flag.Parse()

	db, err := sql.Open("pgx", *dsn)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("dialect: %v", err)
	}
	if err := goose.Up(db, *dir); err != nil {
		log.Fatalf("up: %v", err)
	}
	log.Println("migrations applied")
}
