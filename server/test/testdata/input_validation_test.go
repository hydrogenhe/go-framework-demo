package testdata

import (
	"context"
	"go-framework-demo/server/test/support"
	"strings"
	"testing"

	appconst "go-framework-demo/const"
	"go-framework-demo/server/order_server"
	"go-framework-demo/server/product_server"
	"go-framework-demo/server/user_server"
)

// TestServiceRejectsInvalidInputBeforeDB 验证注册、商品和订单的非法输入在访问数据库前被业务层拒绝。
func TestServiceRejectsInvalidInputBeforeDB(t *testing.T) {
	ctx := context.Background()
	users := user_server.NewUserService(ctx)
	userCases := []support.UserCreateInput{
		{
			Name:     " ",
			Age:      18,
			Email:    "12345@qq.com",
			Password: "123456",
		},
		{
			Name:     "test",
			Age:      0,
			Email:    "12345@qq.com",
			Password: "123456",
		},
		{
			Name:     "test",
			Age:      18,
			Email:    "bad",
			Password: "123456",
		},
		{
			Name:     "test",
			Age:      18,
			Email:    "12345@qq.com",
			Password: "12345",
		},
		{
			Name:  "test",
			Age:   18,
			Email: "12345@qq.com",
			Password: strings.Repeat("a",
				19),
		},
	}
	for _, input := range userCases {
		_, err := users.CreateUser(ctx, input.Name, input.Age, input.Email, input.Password)
		if msg, ok := appconst.AsMsgError(err); !ok || msg.Code != appconst.CodeInvalidParams {
			t.Fatalf("registration accepted invalid input: %v", err)
		}
	}
	products := product_server.NewProductService(ctx)
	_, err := products.Create(ctx, "product", 0, 1, "")
	if msg, ok := appconst.AsMsgError(err); !ok || msg.Code != appconst.CodeInvalidParams {
		t.Fatalf("invalid price accepted: %v", err)
	}
	stock := int32(-1)
	updates := product_server.ProductUpdateInput{Stock: &stock}
	_, err = products.Update(ctx, 1, &updates)
	if msg, ok := appconst.AsMsgError(err); !ok || msg.Code != appconst.CodeInvalidParams {
		t.Fatalf("invalid stock accepted: %v", err)
	}
	orders := order_server.NewOrderService(ctx)
	inputs := []order_server.OrderItemInput{
		{
			ProductID:    1,
			Quantity:     1,
			UnitPriceFen: 100,
		},
		{
			ProductID:    1,
			Quantity:     2,
			UnitPriceFen: 100,
		},
	}
	_, err = orders.CreateOrderWithItems(ctx, "test", 1, inputs)
	if msg, ok := appconst.AsMsgError(err); !ok || msg.Code != appconst.CodeInvalidParams {
		t.Fatalf("duplicate product accepted: %v", err)
	}
}
