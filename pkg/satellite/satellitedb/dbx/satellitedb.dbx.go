// AUTOGENERATED BY gopkg.in/spacemonkeygo/dbx.v1
// DO NOT EDIT.

package satellitedb

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/mattn/go-sqlite3"
)

// Prevent conditional imports from causing build failures
var _ = strconv.Itoa
var _ = strings.LastIndex
var _ = fmt.Sprint
var _ sync.Mutex

var (
	WrapErr = func(err *Error) error { return err }
	Logger  func(format string, args ...interface{})

	errTooManyRows       = errors.New("too many rows")
	errUnsupportedDriver = errors.New("unsupported driver")
	errEmptyUpdate       = errors.New("empty update")
)

func logError(format string, args ...interface{}) {
	if Logger != nil {
		Logger(format, args...)
	}
}

type ErrorCode int

const (
	ErrorCode_Unknown ErrorCode = iota
	ErrorCode_UnsupportedDriver
	ErrorCode_NoRows
	ErrorCode_TxDone
	ErrorCode_TooManyRows
	ErrorCode_ConstraintViolation
	ErrorCode_EmptyUpdate
)

type Error struct {
	Err         error
	Code        ErrorCode
	Driver      string
	Constraint  string
	QuerySuffix string
}

func (e *Error) Error() string {
	return e.Err.Error()
}

func wrapErr(e *Error) error {
	if WrapErr == nil {
		return e
	}
	return WrapErr(e)
}

func makeErr(err error) error {
	if err == nil {
		return nil
	}
	e := &Error{Err: err}
	switch err {
	case sql.ErrNoRows:
		e.Code = ErrorCode_NoRows
	case sql.ErrTxDone:
		e.Code = ErrorCode_TxDone
	}
	return wrapErr(e)
}

func unsupportedDriver(driver string) error {
	return wrapErr(&Error{
		Err:    errUnsupportedDriver,
		Code:   ErrorCode_UnsupportedDriver,
		Driver: driver,
	})
}

func emptyUpdate() error {
	return wrapErr(&Error{
		Err:  errEmptyUpdate,
		Code: ErrorCode_EmptyUpdate,
	})
}

func tooManyRows(query_suffix string) error {
	return wrapErr(&Error{
		Err:         errTooManyRows,
		Code:        ErrorCode_TooManyRows,
		QuerySuffix: query_suffix,
	})
}

func constraintViolation(err error, constraint string) error {
	return wrapErr(&Error{
		Err:        err,
		Code:       ErrorCode_ConstraintViolation,
		Constraint: constraint,
	})
}

type driver interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

var (
	notAPointer     = errors.New("destination not a pointer")
	lossyConversion = errors.New("lossy conversion")
)

type DB struct {
	*sql.DB
	dbMethods

	Hooks struct {
		Now func() time.Time
	}
}

func Open(driver, source string) (db *DB, err error) {
	var sql_db *sql.DB
	switch driver {
	case "sqlite3":
		sql_db, err = opensqlite3(source)
	default:
		return nil, unsupportedDriver(driver)
	}
	if err != nil {
		return nil, makeErr(err)
	}
	defer func(sql_db *sql.DB) {
		if err != nil {
			sql_db.Close()
		}
	}(sql_db)

	if err := sql_db.Ping(); err != nil {
		return nil, makeErr(err)
	}

	db = &DB{
		DB: sql_db,
	}
	db.Hooks.Now = time.Now

	switch driver {
	case "sqlite3":
		db.dbMethods = newsqlite3(db)
	default:
		return nil, unsupportedDriver(driver)
	}

	return db, nil
}

func (obj *DB) Close() (err error) {
	return obj.makeErr(obj.DB.Close())
}

func (obj *DB) Open(ctx context.Context) (*Tx, error) {
	tx, err := obj.DB.Begin()
	if err != nil {
		return nil, obj.makeErr(err)
	}

	return &Tx{
		Tx:        tx,
		txMethods: obj.wrapTx(tx),
	}, nil
}

func (obj *DB) NewRx() *Rx {
	return &Rx{db: obj}
}

func DeleteAll(ctx context.Context, db *DB) (int64, error) {
	tx, err := db.Open(ctx)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err == nil {
			err = db.makeErr(tx.Commit())
			return
		}

		if err_rollback := tx.Rollback(); err_rollback != nil {
			logError("delete-all: rollback failed: %v", db.makeErr(err_rollback))
		}
	}()
	return tx.deleteAll(ctx)
}

type Tx struct {
	Tx *sql.Tx
	txMethods
}

type dialectTx struct {
	tx *sql.Tx
}

func (tx *dialectTx) Commit() (err error) {
	return makeErr(tx.tx.Commit())
}

func (tx *dialectTx) Rollback() (err error) {
	return makeErr(tx.tx.Rollback())
}

type sqlite3Impl struct {
	db      *DB
	dialect __sqlbundle_sqlite3
	driver  driver
}

func (obj *sqlite3Impl) Rebind(s string) string {
	return obj.dialect.Rebind(s)
}

func (obj *sqlite3Impl) logStmt(stmt string, args ...interface{}) {
	sqlite3LogStmt(stmt, args...)
}

func (obj *sqlite3Impl) makeErr(err error) error {
	constraint, ok := obj.isConstraintError(err)
	if ok {
		return constraintViolation(err, constraint)
	}
	return makeErr(err)
}

type sqlite3DB struct {
	db *DB
	*sqlite3Impl
}

func newsqlite3(db *DB) *sqlite3DB {
	return &sqlite3DB{
		db: db,
		sqlite3Impl: &sqlite3Impl{
			db:     db,
			driver: db.DB,
		},
	}
}

func (obj *sqlite3DB) Schema() string {
	return `CREATE TABLE users (
	id BLOB NOT NULL,
	first_name TEXT NOT NULL,
	last_name TEXT NOT NULL,
	email TEXT NOT NULL,
	password_hash BLOB NOT NULL,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( id ),
	UNIQUE ( email )
);
CREATE TABLE companies (
	id BLOB NOT NULL,
	user_id BLOB NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
	name TEXT NOT NULL,
	address TEXT NOT NULL,
	country TEXT NOT NULL,
	city TEXT NOT NULL,
	state TEXT NOT NULL,
	postal_code TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( id )
);`
}

func (obj *sqlite3DB) wrapTx(tx *sql.Tx) txMethods {
	return &sqlite3Tx{
		dialectTx: dialectTx{tx: tx},
		sqlite3Impl: &sqlite3Impl{
			db:     obj.db,
			driver: tx,
		},
	}
}

type sqlite3Tx struct {
	dialectTx
	*sqlite3Impl
}

func sqlite3LogStmt(stmt string, args ...interface{}) {
	// TODO: render placeholders
	if Logger != nil {
		out := fmt.Sprintf("stmt: %s\nargs: %v\n", stmt, pretty(args))
		Logger(out)
	}
}

type pretty []interface{}

func (p pretty) Format(f fmt.State, c rune) {
	fmt.Fprint(f, "[")
nextval:
	for i, val := range p {
		if i > 0 {
			fmt.Fprint(f, ", ")
		}
		rv := reflect.ValueOf(val)
		if rv.Kind() == reflect.Ptr {
			if rv.IsNil() {
				fmt.Fprint(f, "NULL")
				continue
			}
			val = rv.Elem().Interface()
		}
		switch v := val.(type) {
		case string:
			fmt.Fprintf(f, "%q", v)
		case time.Time:
			fmt.Fprintf(f, "%s", v.Format(time.RFC3339Nano))
		case []byte:
			for _, b := range v {
				if !unicode.IsPrint(rune(b)) {
					fmt.Fprintf(f, "%#x", v)
					continue nextval
				}
			}
			fmt.Fprintf(f, "%q", v)
		default:
			fmt.Fprintf(f, "%v", v)
		}
	}
	fmt.Fprint(f, "]")
}

type User struct {
	Id           []byte
	FirstName    string
	LastName     string
	Email        string
	PasswordHash []byte
	CreatedAt    time.Time
}

func (User) _Table() string { return "users" }

type User_Update_Fields struct {
	FirstName    User_FirstName_Field
	LastName     User_LastName_Field
	Email        User_Email_Field
	PasswordHash User_PasswordHash_Field
}

type User_Id_Field struct {
	_set   bool
	_value []byte
}

func User_Id(v []byte) User_Id_Field {
	return User_Id_Field{_set: true, _value: v}
}

func (f User_Id_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (User_Id_Field) _Column() string { return "id" }

type User_FirstName_Field struct {
	_set   bool
	_value string
}

func User_FirstName(v string) User_FirstName_Field {
	return User_FirstName_Field{_set: true, _value: v}
}

func (f User_FirstName_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (User_FirstName_Field) _Column() string { return "first_name" }

type User_LastName_Field struct {
	_set   bool
	_value string
}

func User_LastName(v string) User_LastName_Field {
	return User_LastName_Field{_set: true, _value: v}
}

func (f User_LastName_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (User_LastName_Field) _Column() string { return "last_name" }

type User_Email_Field struct {
	_set   bool
	_value string
}

func User_Email(v string) User_Email_Field {
	return User_Email_Field{_set: true, _value: v}
}

func (f User_Email_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (User_Email_Field) _Column() string { return "email" }

type User_PasswordHash_Field struct {
	_set   bool
	_value []byte
}

func User_PasswordHash(v []byte) User_PasswordHash_Field {
	return User_PasswordHash_Field{_set: true, _value: v}
}

func (f User_PasswordHash_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (User_PasswordHash_Field) _Column() string { return "password_hash" }

type User_CreatedAt_Field struct {
	_set   bool
	_value time.Time
}

func User_CreatedAt(v time.Time) User_CreatedAt_Field {
	return User_CreatedAt_Field{_set: true, _value: v}
}

func (f User_CreatedAt_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (User_CreatedAt_Field) _Column() string { return "created_at" }

type Company struct {
	Id         []byte
	UserId     []byte
	Name       string
	Address    string
	Country    string
	City       string
	State      string
	PostalCode string
	CreatedAt  time.Time
}

func (Company) _Table() string { return "companies" }

type Company_Update_Fields struct {
	Name       Company_Name_Field
	Address    Company_Address_Field
	Country    Company_Country_Field
	City       Company_City_Field
	State      Company_State_Field
	PostalCode Company_PostalCode_Field
}

type Company_Id_Field struct {
	_set   bool
	_value []byte
}

func Company_Id(v []byte) Company_Id_Field {
	return Company_Id_Field{_set: true, _value: v}
}

func (f Company_Id_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (Company_Id_Field) _Column() string { return "id" }

type Company_UserId_Field struct {
	_set   bool
	_value []byte
}

func Company_UserId(v []byte) Company_UserId_Field {
	return Company_UserId_Field{_set: true, _value: v}
}

func (f Company_UserId_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (Company_UserId_Field) _Column() string { return "user_id" }

type Company_Name_Field struct {
	_set   bool
	_value string
}

func Company_Name(v string) Company_Name_Field {
	return Company_Name_Field{_set: true, _value: v}
}

func (f Company_Name_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (Company_Name_Field) _Column() string { return "name" }

type Company_Address_Field struct {
	_set   bool
	_value string
}

func Company_Address(v string) Company_Address_Field {
	return Company_Address_Field{_set: true, _value: v}
}

func (f Company_Address_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (Company_Address_Field) _Column() string { return "address" }

type Company_Country_Field struct {
	_set   bool
	_value string
}

func Company_Country(v string) Company_Country_Field {
	return Company_Country_Field{_set: true, _value: v}
}

func (f Company_Country_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (Company_Country_Field) _Column() string { return "country" }

type Company_City_Field struct {
	_set   bool
	_value string
}

func Company_City(v string) Company_City_Field {
	return Company_City_Field{_set: true, _value: v}
}

func (f Company_City_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (Company_City_Field) _Column() string { return "city" }

type Company_State_Field struct {
	_set   bool
	_value string
}

func Company_State(v string) Company_State_Field {
	return Company_State_Field{_set: true, _value: v}
}

func (f Company_State_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (Company_State_Field) _Column() string { return "state" }

type Company_PostalCode_Field struct {
	_set   bool
	_value string
}

func Company_PostalCode(v string) Company_PostalCode_Field {
	return Company_PostalCode_Field{_set: true, _value: v}
}

func (f Company_PostalCode_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (Company_PostalCode_Field) _Column() string { return "postal_code" }

type Company_CreatedAt_Field struct {
	_set   bool
	_value time.Time
}

func Company_CreatedAt(v time.Time) Company_CreatedAt_Field {
	return Company_CreatedAt_Field{_set: true, _value: v}
}

func (f Company_CreatedAt_Field) value() interface{} {
	if !f._set {
		return nil
	}
	return f._value
}

func (Company_CreatedAt_Field) _Column() string { return "created_at" }

func toUTC(t time.Time) time.Time {
	return t.UTC()
}

func toDate(t time.Time) time.Time {
	// keep up the minute portion so that translations between timezones will
	// continue to reflect properly.
	return t.Truncate(time.Minute)
}

//
// runtime support for building sql statements
//

type __sqlbundle_SQL interface {
	Render() string

	private()
}

type __sqlbundle_Dialect interface {
	Rebind(sql string) string
}

type __sqlbundle_RenderOp int

const (
	__sqlbundle_NoFlatten __sqlbundle_RenderOp = iota
	__sqlbundle_NoTerminate
)

func __sqlbundle_Render(dialect __sqlbundle_Dialect, sql __sqlbundle_SQL, ops ...__sqlbundle_RenderOp) string {
	out := sql.Render()

	flatten := true
	terminate := true
	for _, op := range ops {
		switch op {
		case __sqlbundle_NoFlatten:
			flatten = false
		case __sqlbundle_NoTerminate:
			terminate = false
		}
	}

	if flatten {
		out = __sqlbundle_flattenSQL(out)
	}
	if terminate {
		out += ";"
	}

	return dialect.Rebind(out)
}

var __sqlbundle_reSpace = regexp.MustCompile(`\s+`)

func __sqlbundle_flattenSQL(s string) string {
	return strings.TrimSpace(__sqlbundle_reSpace.ReplaceAllString(s, " "))
}

// this type is specially named to match up with the name returned by the
// dialect impl in the sql package.
type __sqlbundle_postgres struct{}

func (p __sqlbundle_postgres) Rebind(sql string) string {
	out := make([]byte, 0, len(sql)+10)

	j := 1
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		if ch != '?' {
			out = append(out, ch)
			continue
		}

		out = append(out, '$')
		out = append(out, strconv.Itoa(j)...)
		j++
	}

	return string(out)
}

// this type is specially named to match up with the name returned by the
// dialect impl in the sql package.
type __sqlbundle_sqlite3 struct{}

func (s __sqlbundle_sqlite3) Rebind(sql string) string {
	return sql
}

type __sqlbundle_Literal string

func (__sqlbundle_Literal) private() {}

func (l __sqlbundle_Literal) Render() string { return string(l) }

type __sqlbundle_Literals struct {
	Join string
	SQLs []__sqlbundle_SQL
}

func (__sqlbundle_Literals) private() {}

func (l __sqlbundle_Literals) Render() string {
	var out bytes.Buffer

	first := true
	for _, sql := range l.SQLs {
		if sql == nil {
			continue
		}
		if !first {
			out.WriteString(l.Join)
		}
		first = false
		out.WriteString(sql.Render())
	}

	return out.String()
}

type __sqlbundle_Condition struct {
	// set at compile/embed time
	Name  string
	Left  string
	Equal bool
	Right string

	// set at runtime
	Null bool
}

func (*__sqlbundle_Condition) private() {}

func (c *__sqlbundle_Condition) Render() string {

	switch {
	case c.Equal && c.Null:
		return c.Left + " is null"
	case c.Equal && !c.Null:
		return c.Left + " = " + c.Right
	case !c.Equal && c.Null:
		return c.Left + " is not null"
	case !c.Equal && !c.Null:
		return c.Left + " != " + c.Right
	default:
		panic("unhandled case")
	}
}

type __sqlbundle_Hole struct {
	// set at compiile/embed time
	Name string

	// set at runtime
	SQL __sqlbundle_SQL
}

func (*__sqlbundle_Hole) private() {}

func (h *__sqlbundle_Hole) Render() string { return h.SQL.Render() }

//
// end runtime support for building sql statements
//

func (obj *sqlite3Impl) Create_User(ctx context.Context,
	user_id User_Id_Field,
	user_first_name User_FirstName_Field,
	user_last_name User_LastName_Field,
	user_email User_Email_Field,
	user_password_hash User_PasswordHash_Field) (
	user *User, err error) {

	__now := obj.db.Hooks.Now().UTC()
	__id_val := user_id.value()
	__first_name_val := user_first_name.value()
	__last_name_val := user_last_name.value()
	__email_val := user_email.value()
	__password_hash_val := user_password_hash.value()
	__created_at_val := __now

	var __embed_stmt = __sqlbundle_Literal("INSERT INTO users ( id, first_name, last_name, email, password_hash, created_at ) VALUES ( ?, ?, ?, ?, ?, ? )")

	var __stmt = __sqlbundle_Render(obj.dialect, __embed_stmt)
	obj.logStmt(__stmt, __id_val, __first_name_val, __last_name_val, __email_val, __password_hash_val, __created_at_val)

	__res, err := obj.driver.Exec(__stmt, __id_val, __first_name_val, __last_name_val, __email_val, __password_hash_val, __created_at_val)
	if err != nil {
		return nil, obj.makeErr(err)
	}
	__pk, err := __res.LastInsertId()
	if err != nil {
		return nil, obj.makeErr(err)
	}
	return obj.getLastUser(ctx, __pk)

}

func (obj *sqlite3Impl) Create_Company(ctx context.Context,
	company_id Company_Id_Field,
	company_user_id Company_UserId_Field,
	company_name Company_Name_Field,
	company_address Company_Address_Field,
	company_country Company_Country_Field,
	company_city Company_City_Field,
	company_state Company_State_Field,
	company_postal_code Company_PostalCode_Field) (
	company *Company, err error) {

	__now := obj.db.Hooks.Now().UTC()
	__id_val := company_id.value()
	__user_id_val := company_user_id.value()
	__name_val := company_name.value()
	__address_val := company_address.value()
	__country_val := company_country.value()
	__city_val := company_city.value()
	__state_val := company_state.value()
	__postal_code_val := company_postal_code.value()
	__created_at_val := __now

	var __embed_stmt = __sqlbundle_Literal("INSERT INTO companies ( id, user_id, name, address, country, city, state, postal_code, created_at ) VALUES ( ?, ?, ?, ?, ?, ?, ?, ?, ? )")

	var __stmt = __sqlbundle_Render(obj.dialect, __embed_stmt)
	obj.logStmt(__stmt, __id_val, __user_id_val, __name_val, __address_val, __country_val, __city_val, __state_val, __postal_code_val, __created_at_val)

	__res, err := obj.driver.Exec(__stmt, __id_val, __user_id_val, __name_val, __address_val, __country_val, __city_val, __state_val, __postal_code_val, __created_at_val)
	if err != nil {
		return nil, obj.makeErr(err)
	}
	__pk, err := __res.LastInsertId()
	if err != nil {
		return nil, obj.makeErr(err)
	}
	return obj.getLastCompany(ctx, __pk)

}

func (obj *sqlite3Impl) Get_User_By_Email_And_PasswordHash(ctx context.Context,
	user_email User_Email_Field,
	user_password_hash User_PasswordHash_Field) (
	user *User, err error) {

	var __embed_stmt = __sqlbundle_Literal("SELECT users.id, users.first_name, users.last_name, users.email, users.password_hash, users.created_at FROM users WHERE users.email = ? AND users.password_hash = ?")

	var __values []interface{}
	__values = append(__values, user_email.value(), user_password_hash.value())

	var __stmt = __sqlbundle_Render(obj.dialect, __embed_stmt)
	obj.logStmt(__stmt, __values...)

	user = &User{}
	err = obj.driver.QueryRow(__stmt, __values...).Scan(&user.Id, &user.FirstName, &user.LastName, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, obj.makeErr(err)
	}
	return user, nil

}

func (obj *sqlite3Impl) Get_User_By_Id(ctx context.Context,
	user_id User_Id_Field) (
	user *User, err error) {

	var __embed_stmt = __sqlbundle_Literal("SELECT users.id, users.first_name, users.last_name, users.email, users.password_hash, users.created_at FROM users WHERE users.id = ?")

	var __values []interface{}
	__values = append(__values, user_id.value())

	var __stmt = __sqlbundle_Render(obj.dialect, __embed_stmt)
	obj.logStmt(__stmt, __values...)

	user = &User{}
	err = obj.driver.QueryRow(__stmt, __values...).Scan(&user.Id, &user.FirstName, &user.LastName, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, obj.makeErr(err)
	}
	return user, nil

}

func (obj *sqlite3Impl) Get_Company_By_UserId(ctx context.Context,
	company_user_id Company_UserId_Field) (
	company *Company, err error) {

	var __embed_stmt = __sqlbundle_Literal("SELECT companies.id, companies.user_id, companies.name, companies.address, companies.country, companies.city, companies.state, companies.postal_code, companies.created_at FROM companies WHERE companies.user_id = ? LIMIT 2")

	var __values []interface{}
	__values = append(__values, company_user_id.value())

	var __stmt = __sqlbundle_Render(obj.dialect, __embed_stmt)
	obj.logStmt(__stmt, __values...)

	__rows, err := obj.driver.Query(__stmt, __values...)
	if err != nil {
		return nil, obj.makeErr(err)
	}
	defer __rows.Close()

	if !__rows.Next() {
		if err := __rows.Err(); err != nil {
			return nil, obj.makeErr(err)
		}
		return nil, makeErr(sql.ErrNoRows)
	}

	company = &Company{}
	err = __rows.Scan(&company.Id, &company.UserId, &company.Name, &company.Address, &company.Country, &company.City, &company.State, &company.PostalCode, &company.CreatedAt)
	if err != nil {
		return nil, obj.makeErr(err)
	}

	if __rows.Next() {
		return nil, tooManyRows("Company_By_UserId")
	}

	if err := __rows.Err(); err != nil {
		return nil, obj.makeErr(err)
	}

	return company, nil

}

func (obj *sqlite3Impl) Get_Company_By_Id(ctx context.Context,
	company_id Company_Id_Field) (
	company *Company, err error) {

	var __embed_stmt = __sqlbundle_Literal("SELECT companies.id, companies.user_id, companies.name, companies.address, companies.country, companies.city, companies.state, companies.postal_code, companies.created_at FROM companies WHERE companies.id = ?")

	var __values []interface{}
	__values = append(__values, company_id.value())

	var __stmt = __sqlbundle_Render(obj.dialect, __embed_stmt)
	obj.logStmt(__stmt, __values...)

	company = &Company{}
	err = obj.driver.QueryRow(__stmt, __values...).Scan(&company.Id, &company.UserId, &company.Name, &company.Address, &company.Country, &company.City, &company.State, &company.PostalCode, &company.CreatedAt)
	if err != nil {
		return nil, obj.makeErr(err)
	}
	return company, nil

}

func (obj *sqlite3Impl) Update_User_By_Id(ctx context.Context,
	user_id User_Id_Field,
	update User_Update_Fields) (
	user *User, err error) {
	var __sets = &__sqlbundle_Hole{}

	var __embed_stmt = __sqlbundle_Literals{Join: "", SQLs: []__sqlbundle_SQL{__sqlbundle_Literal("UPDATE users SET "), __sets, __sqlbundle_Literal(" WHERE users.id = ?")}}

	__sets_sql := __sqlbundle_Literals{Join: ", "}
	var __values []interface{}
	var __args []interface{}

	if update.FirstName._set {
		__values = append(__values, update.FirstName.value())
		__sets_sql.SQLs = append(__sets_sql.SQLs, __sqlbundle_Literal("first_name = ?"))
	}

	if update.LastName._set {
		__values = append(__values, update.LastName.value())
		__sets_sql.SQLs = append(__sets_sql.SQLs, __sqlbundle_Literal("last_name = ?"))
	}

	if update.Email._set {
		__values = append(__values, update.Email.value())
		__sets_sql.SQLs = append(__sets_sql.SQLs, __sqlbundle_Literal("email = ?"))
	}

	if update.PasswordHash._set {
		__values = append(__values, update.PasswordHash.value())
		__sets_sql.SQLs = append(__sets_sql.SQLs, __sqlbundle_Literal("password_hash = ?"))
	}

	if len(__sets_sql.SQLs) == 0 {
		return nil, emptyUpdate()
	}

	__args = append(__args, user_id.value())

	__values = append(__values, __args...)
	__sets.SQL = __sets_sql

	var __stmt = __sqlbundle_Render(obj.dialect, __embed_stmt)
	obj.logStmt(__stmt, __values...)

	user = &User{}
	_, err = obj.driver.Exec(__stmt, __values...)
	if err != nil {
		return nil, obj.makeErr(err)
	}

	var __embed_stmt_get = __sqlbundle_Literal("SELECT users.id, users.first_name, users.last_name, users.email, users.password_hash, users.created_at FROM users WHERE users.id = ?")

	var __stmt_get = __sqlbundle_Render(obj.dialect, __embed_stmt_get)
	obj.logStmt("(IMPLIED) "+__stmt_get, __args...)

	err = obj.driver.QueryRow(__stmt_get, __args...).Scan(&user.Id, &user.FirstName, &user.LastName, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, obj.makeErr(err)
	}
	return user, nil
}

func (obj *sqlite3Impl) Update_Company_By_Id(ctx context.Context,
	company_id Company_Id_Field,
	update Company_Update_Fields) (
	company *Company, err error) {
	var __sets = &__sqlbundle_Hole{}

	var __embed_stmt = __sqlbundle_Literals{Join: "", SQLs: []__sqlbundle_SQL{__sqlbundle_Literal("UPDATE companies SET "), __sets, __sqlbundle_Literal(" WHERE companies.id = ?")}}

	__sets_sql := __sqlbundle_Literals{Join: ", "}
	var __values []interface{}
	var __args []interface{}

	if update.Name._set {
		__values = append(__values, update.Name.value())
		__sets_sql.SQLs = append(__sets_sql.SQLs, __sqlbundle_Literal("name = ?"))
	}

	if update.Address._set {
		__values = append(__values, update.Address.value())
		__sets_sql.SQLs = append(__sets_sql.SQLs, __sqlbundle_Literal("address = ?"))
	}

	if update.Country._set {
		__values = append(__values, update.Country.value())
		__sets_sql.SQLs = append(__sets_sql.SQLs, __sqlbundle_Literal("country = ?"))
	}

	if update.City._set {
		__values = append(__values, update.City.value())
		__sets_sql.SQLs = append(__sets_sql.SQLs, __sqlbundle_Literal("city = ?"))
	}

	if update.State._set {
		__values = append(__values, update.State.value())
		__sets_sql.SQLs = append(__sets_sql.SQLs, __sqlbundle_Literal("state = ?"))
	}

	if update.PostalCode._set {
		__values = append(__values, update.PostalCode.value())
		__sets_sql.SQLs = append(__sets_sql.SQLs, __sqlbundle_Literal("postal_code = ?"))
	}

	if len(__sets_sql.SQLs) == 0 {
		return nil, emptyUpdate()
	}

	__args = append(__args, company_id.value())

	__values = append(__values, __args...)
	__sets.SQL = __sets_sql

	var __stmt = __sqlbundle_Render(obj.dialect, __embed_stmt)
	obj.logStmt(__stmt, __values...)

	company = &Company{}
	_, err = obj.driver.Exec(__stmt, __values...)
	if err != nil {
		return nil, obj.makeErr(err)
	}

	var __embed_stmt_get = __sqlbundle_Literal("SELECT companies.id, companies.user_id, companies.name, companies.address, companies.country, companies.city, companies.state, companies.postal_code, companies.created_at FROM companies WHERE companies.id = ?")

	var __stmt_get = __sqlbundle_Render(obj.dialect, __embed_stmt_get)
	obj.logStmt("(IMPLIED) "+__stmt_get, __args...)

	err = obj.driver.QueryRow(__stmt_get, __args...).Scan(&company.Id, &company.UserId, &company.Name, &company.Address, &company.Country, &company.City, &company.State, &company.PostalCode, &company.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, obj.makeErr(err)
	}
	return company, nil
}

func (obj *sqlite3Impl) Delete_User_By_Id(ctx context.Context,
	user_id User_Id_Field) (
	deleted bool, err error) {

	var __embed_stmt = __sqlbundle_Literal("DELETE FROM users WHERE users.id = ?")

	var __values []interface{}
	__values = append(__values, user_id.value())

	var __stmt = __sqlbundle_Render(obj.dialect, __embed_stmt)
	obj.logStmt(__stmt, __values...)

	__res, err := obj.driver.Exec(__stmt, __values...)
	if err != nil {
		return false, obj.makeErr(err)
	}

	__count, err := __res.RowsAffected()
	if err != nil {
		return false, obj.makeErr(err)
	}

	return __count > 0, nil

}

func (obj *sqlite3Impl) Delete_Company_By_Id(ctx context.Context,
	company_id Company_Id_Field) (
	deleted bool, err error) {

	var __embed_stmt = __sqlbundle_Literal("DELETE FROM companies WHERE companies.id = ?")

	var __values []interface{}
	__values = append(__values, company_id.value())

	var __stmt = __sqlbundle_Render(obj.dialect, __embed_stmt)
	obj.logStmt(__stmt, __values...)

	__res, err := obj.driver.Exec(__stmt, __values...)
	if err != nil {
		return false, obj.makeErr(err)
	}

	__count, err := __res.RowsAffected()
	if err != nil {
		return false, obj.makeErr(err)
	}

	return __count > 0, nil

}

func (obj *sqlite3Impl) getLastUser(ctx context.Context,
	pk int64) (
	user *User, err error) {

	var __embed_stmt = __sqlbundle_Literal("SELECT users.id, users.first_name, users.last_name, users.email, users.password_hash, users.created_at FROM users WHERE _rowid_ = ?")

	var __stmt = __sqlbundle_Render(obj.dialect, __embed_stmt)
	obj.logStmt(__stmt, pk)

	user = &User{}
	err = obj.driver.QueryRow(__stmt, pk).Scan(&user.Id, &user.FirstName, &user.LastName, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, obj.makeErr(err)
	}
	return user, nil

}

func (obj *sqlite3Impl) getLastCompany(ctx context.Context,
	pk int64) (
	company *Company, err error) {

	var __embed_stmt = __sqlbundle_Literal("SELECT companies.id, companies.user_id, companies.name, companies.address, companies.country, companies.city, companies.state, companies.postal_code, companies.created_at FROM companies WHERE _rowid_ = ?")

	var __stmt = __sqlbundle_Render(obj.dialect, __embed_stmt)
	obj.logStmt(__stmt, pk)

	company = &Company{}
	err = obj.driver.QueryRow(__stmt, pk).Scan(&company.Id, &company.UserId, &company.Name, &company.Address, &company.Country, &company.City, &company.State, &company.PostalCode, &company.CreatedAt)
	if err != nil {
		return nil, obj.makeErr(err)
	}
	return company, nil

}

func (impl sqlite3Impl) isConstraintError(err error) (
	constraint string, ok bool) {
	if e, ok := err.(sqlite3.Error); ok {
		if e.Code == sqlite3.ErrConstraint {
			msg := err.Error()
			colon := strings.LastIndex(msg, ":")
			if colon != -1 {
				return strings.TrimSpace(msg[colon:]), true
			}
			return "", true
		}
	}
	return "", false
}

func (obj *sqlite3Impl) deleteAll(ctx context.Context) (count int64, err error) {
	var __res sql.Result
	var __count int64
	__res, err = obj.driver.Exec("DELETE FROM companies;")
	if err != nil {
		return 0, obj.makeErr(err)
	}

	__count, err = __res.RowsAffected()
	if err != nil {
		return 0, obj.makeErr(err)
	}
	count += __count
	__res, err = obj.driver.Exec("DELETE FROM users;")
	if err != nil {
		return 0, obj.makeErr(err)
	}

	__count, err = __res.RowsAffected()
	if err != nil {
		return 0, obj.makeErr(err)
	}
	count += __count

	return count, nil

}

type Rx struct {
	db *DB
	tx *Tx
}

func (rx *Rx) UnsafeTx(ctx context.Context) (unsafe_tx *sql.Tx, err error) {
	tx, err := rx.getTx(ctx)
	if err != nil {
		return nil, err
	}
	return tx.Tx, nil
}

func (rx *Rx) getTx(ctx context.Context) (tx *Tx, err error) {
	if rx.tx == nil {
		if rx.tx, err = rx.db.Open(ctx); err != nil {
			return nil, err
		}
	}
	return rx.tx, nil
}

func (rx *Rx) Rebind(s string) string {
	return rx.db.Rebind(s)
}

func (rx *Rx) Commit() (err error) {
	if rx.tx != nil {
		err = rx.tx.Commit()
		rx.tx = nil
	}
	return err
}

func (rx *Rx) Rollback() (err error) {
	if rx.tx != nil {
		err = rx.tx.Rollback()
		rx.tx = nil
	}
	return err
}

func (rx *Rx) Create_Company(ctx context.Context,
	company_id Company_Id_Field,
	company_user_id Company_UserId_Field,
	company_name Company_Name_Field,
	company_address Company_Address_Field,
	company_country Company_Country_Field,
	company_city Company_City_Field,
	company_state Company_State_Field,
	company_postal_code Company_PostalCode_Field) (
	company *Company, err error) {
	var tx *Tx
	if tx, err = rx.getTx(ctx); err != nil {
		return
	}
	return tx.Create_Company(ctx, company_id, company_user_id, company_name, company_address, company_country, company_city, company_state, company_postal_code)

}

func (rx *Rx) Create_User(ctx context.Context,
	user_id User_Id_Field,
	user_first_name User_FirstName_Field,
	user_last_name User_LastName_Field,
	user_email User_Email_Field,
	user_password_hash User_PasswordHash_Field) (
	user *User, err error) {
	var tx *Tx
	if tx, err = rx.getTx(ctx); err != nil {
		return
	}
	return tx.Create_User(ctx, user_id, user_first_name, user_last_name, user_email, user_password_hash)

}

func (rx *Rx) Delete_Company_By_Id(ctx context.Context,
	company_id Company_Id_Field) (
	deleted bool, err error) {
	var tx *Tx
	if tx, err = rx.getTx(ctx); err != nil {
		return
	}
	return tx.Delete_Company_By_Id(ctx, company_id)
}

func (rx *Rx) Delete_User_By_Id(ctx context.Context,
	user_id User_Id_Field) (
	deleted bool, err error) {
	var tx *Tx
	if tx, err = rx.getTx(ctx); err != nil {
		return
	}
	return tx.Delete_User_By_Id(ctx, user_id)
}

func (rx *Rx) Get_Company_By_Id(ctx context.Context,
	company_id Company_Id_Field) (
	company *Company, err error) {
	var tx *Tx
	if tx, err = rx.getTx(ctx); err != nil {
		return
	}
	return tx.Get_Company_By_Id(ctx, company_id)
}

func (rx *Rx) Get_Company_By_UserId(ctx context.Context,
	company_user_id Company_UserId_Field) (
	company *Company, err error) {
	var tx *Tx
	if tx, err = rx.getTx(ctx); err != nil {
		return
	}
	return tx.Get_Company_By_UserId(ctx, company_user_id)
}

func (rx *Rx) Get_User_By_Email_And_PasswordHash(ctx context.Context,
	user_email User_Email_Field,
	user_password_hash User_PasswordHash_Field) (
	user *User, err error) {
	var tx *Tx
	if tx, err = rx.getTx(ctx); err != nil {
		return
	}
	return tx.Get_User_By_Email_And_PasswordHash(ctx, user_email, user_password_hash)
}

func (rx *Rx) Get_User_By_Id(ctx context.Context,
	user_id User_Id_Field) (
	user *User, err error) {
	var tx *Tx
	if tx, err = rx.getTx(ctx); err != nil {
		return
	}
	return tx.Get_User_By_Id(ctx, user_id)
}

func (rx *Rx) Update_Company_By_Id(ctx context.Context,
	company_id Company_Id_Field,
	update Company_Update_Fields) (
	company *Company, err error) {
	var tx *Tx
	if tx, err = rx.getTx(ctx); err != nil {
		return
	}
	return tx.Update_Company_By_Id(ctx, company_id, update)
}

func (rx *Rx) Update_User_By_Id(ctx context.Context,
	user_id User_Id_Field,
	update User_Update_Fields) (
	user *User, err error) {
	var tx *Tx
	if tx, err = rx.getTx(ctx); err != nil {
		return
	}
	return tx.Update_User_By_Id(ctx, user_id, update)
}

type Methods interface {
	Create_Company(ctx context.Context,
		company_id Company_Id_Field,
		company_user_id Company_UserId_Field,
		company_name Company_Name_Field,
		company_address Company_Address_Field,
		company_country Company_Country_Field,
		company_city Company_City_Field,
		company_state Company_State_Field,
		company_postal_code Company_PostalCode_Field) (
		company *Company, err error)

	Create_User(ctx context.Context,
		user_id User_Id_Field,
		user_first_name User_FirstName_Field,
		user_last_name User_LastName_Field,
		user_email User_Email_Field,
		user_password_hash User_PasswordHash_Field) (
		user *User, err error)

	Delete_Company_By_Id(ctx context.Context,
		company_id Company_Id_Field) (
		deleted bool, err error)

	Delete_User_By_Id(ctx context.Context,
		user_id User_Id_Field) (
		deleted bool, err error)

	Get_Company_By_Id(ctx context.Context,
		company_id Company_Id_Field) (
		company *Company, err error)

	Get_Company_By_UserId(ctx context.Context,
		company_user_id Company_UserId_Field) (
		company *Company, err error)

	Get_User_By_Email_And_PasswordHash(ctx context.Context,
		user_email User_Email_Field,
		user_password_hash User_PasswordHash_Field) (
		user *User, err error)

	Get_User_By_Id(ctx context.Context,
		user_id User_Id_Field) (
		user *User, err error)

	Update_Company_By_Id(ctx context.Context,
		company_id Company_Id_Field,
		update Company_Update_Fields) (
		company *Company, err error)

	Update_User_By_Id(ctx context.Context,
		user_id User_Id_Field,
		update User_Update_Fields) (
		user *User, err error)
}

type TxMethods interface {
	Methods

	Rebind(s string) string
	Commit() error
	Rollback() error
}

type txMethods interface {
	TxMethods

	deleteAll(ctx context.Context) (int64, error)
	makeErr(err error) error
}

type DBMethods interface {
	Methods

	Schema() string
	Rebind(sql string) string
}

type dbMethods interface {
	DBMethods

	wrapTx(tx *sql.Tx) txMethods
	makeErr(err error) error
}

var sqlite3DriverName = "sqlite3_" + fmt.Sprint(time.Now().UnixNano())

func init() {
	sql.Register(sqlite3DriverName, &sqlite3.SQLiteDriver{
		ConnectHook: sqlite3SetupConn,
	})
}

// SQLite3JournalMode controls the journal_mode pragma for all new connections.
// Since it is read without a mutex, it must be changed to the value you want
// before any Open calls.
var SQLite3JournalMode = "WAL"

func sqlite3SetupConn(conn *sqlite3.SQLiteConn) (err error) {
	_, err = conn.Exec("PRAGMA foreign_keys = ON", nil)
	if err != nil {
		return makeErr(err)
	}
	_, err = conn.Exec("PRAGMA journal_mode = "+SQLite3JournalMode, nil)
	if err != nil {
		return makeErr(err)
	}
	return nil
}

func opensqlite3(source string) (*sql.DB, error) {
	return sql.Open(sqlite3DriverName, source)
}
