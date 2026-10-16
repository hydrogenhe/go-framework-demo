package db

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go-framework-demo/query"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestExpiredPageUsesTimeAndIDCursor(t *testing.T) {
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
	orders, err := NewOrderDb(context.Background(), query.Use(database))
	if err != nil {
		t.Fatal(err)
	}
	cutoff := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	after := cutoff.Add(-time.Minute)
	input := ExpiredOrderQuery{Status: 1, Cutoff: cutoff, AfterExpireTime: after, AfterID: 20, Limit: 50}
	mock.ExpectQuery("SELECT `order`.`id`,`order`.`expire_time` FROM `order` WHERE .*expire_time.* OR .* ORDER BY `order`.`expire_time`,`order`.`id` LIMIT \\?").
		WithArgs(int32(1), cutoff, after, after, int64(20), 0, 50).
		WillReturnRows(sqlmock.NewRows([]string{"id", "expire_time"}).AddRow(30, after).AddRow(5, after.Add(time.Second)))
	rows, err := orders.FindExpiredPage(context.Background(), &input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != 30 || rows[1].ID != 5 {
		t.Fatalf("unexpected rows=%v", rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
