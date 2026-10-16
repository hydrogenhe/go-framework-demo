package test

import (
	"testing"
	"time"

	appconst "go-framework-demo/const"
	"go-framework-demo/es"
	"go-framework-demo/es/es_model"
	"go-framework-demo/util"
)

// TestESOrderCRUD 使用真实 ES，验证订单文档创建、查询、明细和状态更新及创建时间戳。
func TestESOrderCRUD(t *testing.T) {
	if err := es.InitES(); err != nil {
		t.Skipf("ES不可用，跳过测试: %v", err)
	}
	order := &es_model.Order{ID: util.GetID(), OrderNo: "O_ES_" + util.GenerateAlnum(8), UserID: util.GetID(), AmountFen: 19900, Status: appconst.OrderStatusPaid, CreateTime: time.Now(), Items: []es_model.OrderItem{{ProductID: util.GetID(), ProductName: "测试商品A", Quantity: 2, UnitPriceFen: 5000, Status: appconst.OrderItemStatusPendingShipment}, {ProductID: util.GetID(), ProductName: "测试商品B", Quantity: 1, UnitPriceFen: 9900, Status: appconst.OrderItemStatusPendingShipment}}}
	docID, err := es.IndexOrder(order)
	if err != nil {
		t.Fatalf("写入Order失败: %v", err)
	}
	t.Logf("Order写入成功，文档ID：%s", docID)

	actualOrder, err := es.GetOrderByID(order.ID)
	if err != nil {
		t.Fatalf("查询Order失败: %v", err)
	}
	if actualOrder.OrderNo != order.OrderNo || actualOrder.UserID != order.UserID || actualOrder.AmountFen != order.AmountFen {
		t.Fatalf("查询订单数据不匹配: %+v", actualOrder)
	}
	if len(actualOrder.Items) != len(order.Items) {
		t.Fatalf("订单商品明细数量不匹配")
	}
	if actualOrder.CreateTimeStamp != order.CreateTime.UnixMilli() {
		t.Errorf("订单创建时间戳不匹配")
	}
	order.Status = appconst.OrderStatusCompleted
	if err := es.UpdateOrder(order); err != nil {
		t.Fatalf("更新Order失败: %v", err)
	}
	updatedOrder, err := es.GetOrderByID(order.ID)
	if err != nil {
		t.Fatalf("查询更新后的Order失败: %v", err)
	}
	if updatedOrder.Status != order.Status {
		t.Errorf("Order更新失败，预期：%d，实际：%d", order.Status, updatedOrder.Status)
	}
}
