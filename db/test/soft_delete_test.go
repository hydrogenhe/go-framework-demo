package test

import (
	"context"
	"errors"
	"go-framework-demo/db"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/query"
	"go-framework-demo/util"
	"gorm.io/gorm"
	"os"
	"testing"
	"time"
)

func TestSoftDeleteDefaultScope(t *testing.T) {
	if os.Getenv("DEMO_INTEGRATION") != "1" {
		t.Skip("set DEMO_INTEGRATION=1")
	}
	mysql.InitMySQL()
	tx := mysql.GetDB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	q := query.Use(tx)
	ctx := context.Background()
	id := util.GetID()
	user := fixtureUser(t, "soft_delete", 18, util.RandomTestEmail(), "password")
	user.ID = id
	product := &model.Product{ID: id, Name: "soft_delete", Price: 1000, Stock: 10, CreateTime: time.Now()}
	order := &model.Order{ID: id, OrderNo: "soft_" + util.GenerateAlnum(8), UserID: id, Amount: 1000, Status: 1, CreateTime: time.Now()}
	item := &model.OrderItem{ID: id, OrderID: id, ProductID: id, Quantity: 1, UnitPriceFen: 1000, Status: 1}
	u, _ := db.NewUserDb(ctx, q)
	p, _ := db.NewProductDb(ctx, q)
	o, _ := db.NewOrderDb(ctx, q)
	i, _ := db.NewOrderItemDb(ctx, q)
	if _, err := u.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Create(ctx, product); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Create(ctx, order); err != nil {
		t.Fatal(err)
	}
	if _, err := i.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		Table  string
		Get    func() error
		Delete func() (bool, error)
	}{
		{"user", func() error { _, e := u.GetByID(ctx, id); return e }, func() (bool, error) { return u.Delete(ctx, id) }},
		{"product", func() error { _, e := p.GetByID(ctx, id); return e }, func() (bool, error) { return p.Delete(ctx, id) }},
		{"order", func() error { _, e := o.GetByID(ctx, id); return e }, func() (bool, error) { return o.Delete(ctx, id) }},
		{"order_item", func() error { _, e := i.GetByID(ctx, id); return e }, func() (bool, error) { return i.Delete(ctx, id) }},
	} {
		if err := input.Get(); err != nil {
			t.Fatalf("%s: default zero invisible: %v", input.Table, err)
		}
		ok, err := input.Delete()
		if err != nil || !ok {
			t.Fatalf("%s delete: %v %v", input.Table, ok, err)
		}
		if err := input.Get(); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("%s soft deleted row visible: %v", input.Table, err)
		}
		var flag int64
		if err := tx.Table(input.Table).Where("id = ?", id).Pluck("is_del", &flag).Error; err != nil || flag != 1 {
			t.Fatalf("%s row not retained with is_del=1: %d %v", input.Table, flag, err)
		}
	}
	if found, e := p.GetByIDIncludingDeleted(ctx, id); e != nil || found.IsDel != 1 {
		t.Fatalf("unscoped read failed: %+v %v", found, e)
	}
	if users, _, e := u.PageQuery(ctx, 1, 100, "soft_delete"); e != nil || len(users) != 0 {
		t.Fatalf("deleted user returned by pagination: %v", e)
	}
	if items, e := i.GetByOrderID(ctx, id); e != nil || len(items) != 0 {
		t.Fatalf("deleted item returned: %v", e)
	}
	if items, e := i.GetByOrderIDIncludingDeleted(ctx, id); e != nil || len(items) != 1 || items[0].IsDel != 1 {
		t.Fatalf("CDC cannot read deleted item: %v", e)
	}
	if affected, e := p.DeductStock(ctx, id, 1, product.CurrentVersion); e != nil || affected != 0 {
		t.Fatalf("deleted product stock updated: %d %v", affected, e)
	}
}
