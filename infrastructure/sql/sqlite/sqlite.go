package sqlite

import (
	"context"
	sql "database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type Api interface {
	Connect()
	Select()
	Insert()
	Update()
	Delete()
	Get()
	Begin()
	Commit()
	Prepare()
	Execute()
	Query()
	OpenDb()
}

type Sqlite struct {
	db   *sql.DB
	tx   *sql.Tx
	stmt *sql.Stmt
	rs   *sql.Rows
}

type Tx struct {
	db          *sql.DB
	transaction *sql.Tx
	stmt        *sql.Stmt
}

//zh-CH :连接数据库,存在直接打开，不存在新建数据库
//en_US
func (sqlite *Sqlite) OpenDb(address string) {
	connect, err := sql.Open("sqlite3", address)
	if err != nil {
		log.Fatal(err)
	}
	sqlite.db = connect
}

func (sqlite *Sqlite) Close() error {
	defer sqlite.db.Close()
}

func (sqlite *Sqlite) Select() {

}

//zh-CH :开启事务
//en_US
func (sqlite *Sqlite) Begin(db *sql.DB) (*sql.Tx, error) {
	tx, err := sqlite.db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	return tx, err
}

func (tx *Tx) Prepare(query string) (*sql.Stmt, error) {
	return tx.transaction.PrepareContext(context.Background(), query)
}

func (sqlite *Sqlite) Execute(args ...interface{}) (sql.Result, error) {
	return sqlite.stmt.ExecContext(context.Background(), args...)
}

//zh-CH :提交事务
//en_US
func (sqlite *Sqlite) Commit(tx *sql.Tx) {
	err := tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}

//zh-CH :sql 语句
//en_US:
// Query executes a query that returns rows, typically a SELECT.
// The args are for any placeholder parameters in the query.
//
// Query uses context.Background internally; to specify the context, use
// QueryContext.
func (sqlite *Sqlite) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return sqlite.db.QueryContext(context.Background(), query, args...)
}
