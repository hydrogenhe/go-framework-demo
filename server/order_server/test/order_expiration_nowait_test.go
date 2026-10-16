package order

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlerr "github.com/go-sql-driver/mysql"
	"go-framework-demo/query"
	"go-framework-demo/server/order_server"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestCloseIfExpiredNowaitAndDatabaseFailure(t *testing.T) {
	cases := []struct {
		name       string
		queryError error
		skipped    bool
		waitLock   bool
	}{
		{"locked order skips", &mysqlerr.MySQLError{Number: 3572, Message: "NOWAIT lock conflict"}, true, false},
		{"ordinary database error rolls back", errors.New("connection failure"), false, false},
		{"waiting lock timeout propagates", &mysqlerr.MySQLError{Number: 1205, Message: "Lock wait timeout"}, false, true},
		{"waiting conflict propagates", &mysqlerr.MySQLError{Number: 3572, Message: "lock conflict"}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			connection, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			driverConfig := mysql.Config{Conn: connection, SkipInitializeWithVersion: true}
			gormConfig := &gorm.Config{DisableAutomaticPing: true}
			database, err := gorm.Open(mysql.New(driverConfig), gormConfig)
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			lockSQL := "SELECT .* FROM `order` WHERE .* FOR UPDATE"
			if !tc.waitLock {
				lockSQL += " NOWAIT"
			}
			mock.ExpectQuery(lockSQL+"$").WithArgs(int64(42), 1).WillReturnError(tc.queryError)
			if tc.skipped {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			service := order_server.NewOrderService(context.Background(), query.Use(database))
			closed, err := service.CloseIfExpired(context.Background(), 42, tc.waitLock)
			if closed || (err == nil) != tc.skipped {
				t.Errorf("closed=%v err=%v", closed, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
