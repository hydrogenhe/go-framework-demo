package http_handler

import (
	"go-framework-demo/http_handler/http_msg"
	"go-framework-demo/model"
	"strconv"

	"github.com/gin-gonic/gin"
	appconst "go-framework-demo/const"
	"go-framework-demo/server/order_server"
)

func orderPathID(c *gin.Context, key string) (int64, error) {
	var path http_msg.OrderPathReq
	if err := c.ShouldBindUri(&path); err != nil {
		return 0, appconst.NewInvalidParamsError("订单ID不能为空")
	}
	raw := path.ID
	if key == "item_id" {
		raw = path.ItemID
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, appconst.NewInvalidParamsError(key + " 必须为正整数")
	}
	return value, nil
}

func (h routeHandlers) payOrder(c *gin.Context) {
	user, err := currentUser(c)
	if err != nil {
		writeError(c, err)
		return
	}
	id, err := orderPathID(c, "id")
	if err != nil {
		writeError(c, err)
		return
	}
	result, err := h.orderService.PayOrder(c.Request.Context(), id, user.ID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeSuccess(c, 200, orderResp(result))
}

func (h routeHandlers) confirmReceipt(c *gin.Context) {
	user, err := currentUser(c)
	if err != nil {
		writeError(c, err)
		return
	}
	id, err := orderPathID(c, "id")
	if err != nil {
		writeError(c, err)
		return
	}
	result, err := h.orderService.ConfirmReceipt(c.Request.Context(), id, user.ID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeSuccess(c, 200, orderResp(result))
}

func (h routeHandlers) shipOrderItem(c *gin.Context) {
	user, err := currentUser(c)
	if err != nil {
		writeError(c, err)
		return
	}
	if !IsAdmin(user) {
		writeError(c, forbiddenError())
		return
	}
	id, err := orderPathID(c, "id")
	if err != nil {
		writeError(c, err)
		return
	}
	itemID, err := orderPathID(c, "item_id")
	if err != nil {
		writeError(c, err)
		return
	}
	result, err := h.orderService.ShipOrderItem(c.Request.Context(), id, itemID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeSuccess(c, 200, orderItemResp(result))
}

func (h routeHandlers) listAdminOrders(c *gin.Context) {
	user, err := currentUser(c)
	if err != nil {
		writeError(c, err)
		return
	}
	if !IsAdmin(user) {
		writeError(c, forbiddenError())
		return
	}
	req, err := bindOrderQuery(c)
	if err != nil {
		writeError(c, appconst.NewInvalidParamsError(err.Error()))
		return
	}
	userID, err := parseOrderUserID(req.UserID, false)
	if err != nil {
		writeError(c, appconst.NewInvalidParamsError(err.Error()))
		return
	}
	input, err := parseOrderQuery(req, userID)
	if err != nil {
		writeError(c, appconst.NewInvalidParamsError(err.Error()))
		return
	}
	result, err := h.orderService.ListAdmin(c.Request.Context(), input)
	if err != nil {
		writeError(c, err)
		return
	}
	writeSuccess(c, 200, orderPageResp(result))
}

func PayOrderHandler(service *order_server.OrderService) gin.HandlerFunc {
	return (routeHandlers{orderService: service}).payOrder
}
func ConfirmReceiptHandler(service *order_server.OrderService) gin.HandlerFunc {
	return (routeHandlers{orderService: service}).confirmReceipt
}
func ShipOrderItemHandler(service *order_server.OrderService) gin.HandlerFunc {
	return (routeHandlers{orderService: service}).shipOrderItem
}
func ListAdminOrdersHandler(service *order_server.OrderService) gin.HandlerFunc {
	return (routeHandlers{orderService: service}).listAdminOrders
}

// orderItemSnapshot 订单商品快照：本人可读，管理员可查看全部订单。
func (h routeHandlers) orderItemSnapshot(c *gin.Context) {
	ctx := c.Request.Context()
	user, err := currentUser(c)
	if err != nil {
		writeError(c, err)
		return
	}
	orderID, err := orderPathID(c, "id")
	if err != nil {
		writeError(c, err)
		return
	}
	itemID, err := orderPathID(c, "item_id")
	if err != nil {
		writeError(c, err)
		return
	}
	var item *model.OrderItem
	if IsAdmin(user) {
		item, err = h.orderService.GetAdminItemSnapshot(ctx, orderID, itemID)
	} else {
		item, err = h.orderService.GetItemSnapshot(ctx, orderID, itemID, user.ID)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	writeSuccess(c, 200, orderItemResp(item))
}

// adminOrderUser 仅管理员可查看订单所属用户的名字和邮箱。
func (h routeHandlers) adminOrderUser(c *gin.Context) {
	ctx := c.Request.Context()
	user, err := currentUser(c)
	if err != nil {
		writeError(c, err)
		return
	}
	if !IsAdmin(user) {
		writeError(c, forbiddenError())
		return
	}
	orderID, err := orderPathID(c, "id")
	if err != nil {
		writeError(c, err)
		return
	}
	detail, err := h.orderService.GetAdminOrderUser(ctx, orderID)
	if err != nil {
		writeError(c, err)
		return
	}
	response := http_msg.OrderUserDetailResp{ID: strconv.FormatInt(detail.ID, 10), Name: detail.Name, Email: detail.Email}
	writeSuccess(c, 200, response)
}

func AdminOrderUserHandler(service *order_server.OrderService) gin.HandlerFunc {
	return (routeHandlers{orderService: service}).adminOrderUser
}
