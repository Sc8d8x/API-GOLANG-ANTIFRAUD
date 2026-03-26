package repository

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// для работы с postgres
type Postgres struct {
	*sql.DB
}

func (p *Postgres) GetDB() *sql.DB {
	return p.DB
}

// создать подключение к базе postgress
func NewPostgres(connSTR string) (*Postgres, error) {
	db, err := sql.Open("postgres", connSTR)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Postgres{DB: db}, nil

}

// закрыть подключение к базе
func (db *Postgres) close() error {
	return db.DB.Close()
}

// возвращаем строку подключение
func ConnectionStr(host, port, user, password, dbname string) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
}
