package http_handler

import (
	"github.com/gin-gonic/gin"
	appconst "go-framework-demo/const"
	"go-framework-demo/http_handler/http_msg"
	"go-framework-demo/server/order_server"
	"net/http"
)

// OrderRequest 表示创建订单接口接收的 JSON 请求数据。
type OrderRequest = http_msg.OrderCreateReq

// CreateOrderHandler 返回创建订单接口的 HTTP 处理函数，供测试或其他调用方复用。
func CreateOrderHandler(s *order_server.OrderService) gin.HandlerFunc {
	return (routeHandlers{orderService: s}).createOrder
}

// 创建订单。
func (h routeHandlers) createOrder(c *gin.Context) {
	ctx := c.Request.Context()
	user, err := currentUser(c)
	if err != nil {
		writeError(c, err)
		return
	}
	var r OrderRequest
	if c.ShouldBindJSON(&r) != nil {
		writeError(c, appconst.NewInvalidParamsError("请求参数格式不正确"))
		return
	}
	if err := validateUserIDAccess(user, r.UserID, false); err != nil {
		writeError(c, err)
		return
	}
	items := make([]order_server.OrderItemInput, 0, len(r.Items))
	for _, item := range r.Items {
		items = append(items, order_server.OrderItemInput{ProductID: item.ProductID, ProductVersion: item.ProductVersion, Quantity: item.Quantity, UnitPriceFen: item.UnitPriceFen, Name: item.Name, Image: item.Image, Stock: item.Stock})
	}
	v, e := h.orderService.CreateOrderWithItems(ctx, r.OrderNo, r.UserID, items)
	if e != nil {
		writeError(c, e)
		return
	}
	writeSuccess(c, http.StatusCreated, orderResp(v))
}

// ListOrdersHandler 返回订单查询接口的 HTTP 处理函数，供测试或其他调用方复用。
func ListOrdersHandler(s *order_server.OrderService) gin.HandlerFunc {
	return (routeHandlers{orderService: s}).listOrders
}

// 查询用户订单。
func (h routeHandlers) listOrders(c *gin.Context) {
	ctx := c.Request.Context()
	user, err := currentUser(c)
	if err != nil {
		writeError(c, err)
		return
	}
	req, err := bindOrderQuery(c)
	if err != nil {
		writeError(c, appconst.NewInvalidParamsError(err.Error()))
		return
	}
	uid, err := parseOrderUserID(req.UserID, true)
	if err != nil {
		writeError(c, appconst.NewInvalidParamsError(err.Error()))
		return
	}
	if err := validateUserIDAccess(user, uid, true); err != nil {
		writeError(c, err)
		return
	}
	input, err := parseOrderQuery(req, uid)
	if err != nil {
		writeError(c, appconst.NewInvalidParamsError(err.Error()))
		return
	}
	v, e := h.orderService.List(ctx, input)
	if e != nil {
		writeError(c, e)
		return
	}
	writeSuccess(c, 200, orderPageResp(v))
}

// 标记订单发货。
func (h routeHandlers) shipOrder(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := orderPathID(c, "id")
	if err != nil {
		writeError(c, err)
		return
	}
	v, e := h.orderService.MarkShipped(ctx, id)
	if e != nil {
		writeError(c, e)
		return
	}
	writeSuccess(c, 200, orderResp(v))
}
