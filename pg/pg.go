package pg

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"reflect"
	"strings"
	"template"
	"time"
)

type DB struct {
	db     *sqlx.DB
	ctx    context.Context // background context
	cancel func()          // cancel background context
	// Datasource name.
	DSN string
	// Returns the current time. Defaults to time.Now().
	// Can be mocked for tests.
	Now func() time.Time
}

// Tx wraps the SQL Tx object to provide a timestamp at the start of the transaction.
type Tx struct {
	*sqlx.Tx
	db  *DB
	now time.Time
}

// NewDB returns a new instance of DB associated with the given datasource name.
func NewDB(dsn string) *DB {
	db := &DB{
		DSN: dsn,
		Now: time.Now,
	}
	db.ctx, db.cancel = context.WithCancel(context.Background())
	return db
}

// Connect to the database.
func (db *DB) Connect() (err error) {
	db.db, err = sqlx.Connect("postgres", db.DSN)

	if err := db.migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return err
}

// Close the database connection.
func (db *DB) Close() (err error) {
	return db.db.Close()
}

// beginTx starts a transaction and returns a wrapper Tx type. This type
// provides a reference to the database and a fixed timestamp at the start of
// the transaction. The timestamp allows us to mock time during tests as well.
func (db *DB) beginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := db.db.BeginTxx(ctx, opts)
	if err != nil {
		return nil, err
	}

	// Return wrapper Tx that includes the transaction start time.
	return &Tx{
		Tx:  tx,
		db:  db,
		now: db.Now().UTC().Truncate(time.Second),
	}, nil
}

// migrate updates the connected database by running any outstanding migration scripts.
func (db *DB) migrate() error {
	driver, err := postgres.WithInstance(db.db.DB, &postgres.Config{})
	m, err := migrate.NewWithDatabaseInstance("file://pg/migrations", "code_snippets", driver)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil {
		if err.Error() != "no change" {
			return err
		}
		err = nil
	}
	return nil
}

// hashString applies the SHA256 hashing algorithm to a string
func hashString(str string) string {
	hasher := sha256.New()
	hasher.Write([]byte(str))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

// formatLimitOffset returns a SQL string for a given limit & offset.
// Clauses are only added if limit and/or offset are greater than zero.
func formatLimitOffset(limit, page int) string {
	if limit > 0 && page > 1 {
		return fmt.Sprintf(`LIMIT %d OFFSET %d`, limit, (page-1)*limit)
	} else if limit > 0 {
		return fmt.Sprintf(`LIMIT %d`, limit)
	}
	return ""
}

func (tx *Tx) buildInQuery(baseQuery string, args ...interface{}) (string, []interface{}, error) {
	q, args, err := sqlx.In(baseQuery, args...)
	if err != nil {
		return "", nil, nil
	}
	q = tx.Rebind(q)

	return q, args, nil
}

// insert an entity into the database. The reflect package is used to build the query from the provided entity. `entity`
// should be a pointer to the struct that should be inserted into the database. It is expected that the underlying
// struct would have the following properties: ID, CreatedAt, UpdatedAt. These properties are set within the function.
func (tx *Tx) insert(ctx context.Context, entity interface{}, table string) error {
	currTime := time.Now()
	// Set metadata for struct to be inserted into the database.
	reflect.Indirect(reflect.ValueOf(entity)).FieldByName("CreatedAt").Set(reflect.ValueOf(currTime))
	reflect.Indirect(reflect.ValueOf(entity)).FieldByName("UpdatedAt").Set(reflect.ValueOf(currTime))

	// Build a slice containing the column names for all fields to be inserted into the database.
	var columns []string

	t := reflect.Indirect(reflect.ValueOf(entity)).Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		// If a field has no `db` tag then the struct is invalid and we should fail
		column, ok := field.Tag.Lookup("db")
		if !ok {
			return template.Errorf(template.EINTERNAL, "Field '%s' does not contain a `db` tag.", field.Name)
		}
		// The tag `id,omitempty` should be used for primary key fields and should be omitted from the query. The tag `-`
		// should be used for related structs and properties that do not have an underlying value persisted in the database.
		// These values should also be omitted from the query.
		if column == "id,omitempty" || column == "-" {
			continue
		}
		columns = append(columns, column)
	}

	// Build the SQL query to be executed from the column names that have been derived from the struct's tags and the
	// `table` argument.
	q := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) RETURNING id;",
		table,
		strings.Join(columns, ", "),
		":"+strings.Join(columns, ", :"),
	)

	// Use the sqlx package to execute the query.
	stmt, err := tx.PrepareNamed(q)

	if err != nil {
		return err
	}

	var id int
	err = stmt.Get(&id, entity)

	// Assign the returned ID value to the entity.
	reflect.Indirect(reflect.ValueOf(entity)).FieldByName("ID").SetInt(int64(id))

	return nil
}

func (tx *Tx) update(ctx context.Context, entity interface{}, table string) error {
	// Set metadata for struct to be inserted into the database.
	reflect.Indirect(reflect.ValueOf(entity)).FieldByName("UpdatedAt").Set(reflect.ValueOf(time.Now()))

	// Build a slice containing the column names for all fields to be updated in the database.
	var columns []string

	t := reflect.Indirect(reflect.ValueOf(entity)).Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		// If a field has no `db` tag then the struct is invalid and we should fail
		column, ok := field.Tag.Lookup("db")
		if !ok {
			return template.Errorf(template.EINTERNAL, "Field '%s' does not contain a `db` tag.", field.Name)
		}
		// The tag `id,omitempty` should be used for primary key fields and should be omitted from the query. The tag `-`
		// should be used for related structs and properties that do not have an underlying value persisted in the database.
		// These values should also be omitted from the query.
		if column == "id,omitempty" || column == "-" || column == "company_id" || column == "created_at" || column == "created_by" {
			continue
		}
		columns = append(columns, column)
	}

	q := fmt.Sprintf("UPDATE %s SET ", table)
	for i, col := range columns {
		q += fmt.Sprintf("%s = :%s", col, col)
		if i == len(columns)-1 {
			q += " "
			break
		}
		q += ", "
	}

	q += "WHERE id = :id"
	_, err := tx.NamedExec(q, entity)

	return err
}
//
//func (tx *Tx) delete(ctx context.Context, id int, table string) error {
//	user := session.UserFromContext(ctx)
//
//	q := fmt.Sprintf("UPDATE %s SET deleted = true, deleter_id = $1, deleted_at = $2 WHERE id = $3 AND company_id = $4", table)
//	_, err := tx.Exec(q, user.ID, time.Now(), id, user.CompanyID)
//
//	return err
//}
//
//func (tx *Tx) get(ctx context.Context, dest interface{}, id int, table string) error {
//	user := session.UserFromContext(ctx)
//
//	q := fmt.Sprintf("SELECT * FROM %s WHERE id = $1", table)
//
//	err := tx.Get(dest, q, id, user.CompanyID)
//
//	return err
//}
