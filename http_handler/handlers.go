package http_handler

import (
	"context"
	"go-framework-demo/logger"
	"go-framework-demo/server"

	"go-framework-demo/server/order_server"
	"go-framework-demo/server/product_server"
	"go-framework-demo/server/user_server"

	"github.com/gin-gonic/gin"
)

// routeHandlers 持有 HTTP 接口依赖的服务。
// 路由注册时只绑定其方法，避免在每条路由上重复传入服务依赖。
type routeHandlers struct {
	userService    *user_server.UserService
	productService *product_server.ProductService
	orderService   *order_server.OrderService
	imageService   *product_server.ImageService
}

func newRouteHandlers(ctx context.Context) routeHandlers {
	userService := user_server.NewUserService(ctx)
	productService := product_server.NewProductService(ctx)
	orderService := order_server.NewOrderService(ctx)
	imageService := product_server.NewImageService()

	handlers := routeHandlers{
		userService:    userService,
		productService: productService,
		orderService:   orderService,
		imageService:   imageService,
	}
	return handlers
}

// RegisterRoutes 初始化路由依赖，并按访问权限注册 HTTP 路由。
func RegisterRoutes(router *gin.Engine) {
	handlers := newRouteHandlers(context.Background())

	// 公开路由：仅注册和登录无需会话。
	router.PUT("/api/auth/register", handlers.createUser) // 注册新用户。
	router.PUT("/api/auth/login", handlers.login)         // 校验用户凭证并创建登录会话。

	// 登录路由：组内接口都必须携带有效登录会话。
	authenticated := router.Group("/api")
	authenticated.Use(RequireAuth())
	authenticated.PUT("/orders/:id/items/:item_id/snapshot", handlers.orderItemSnapshot)
	authenticated.PUT("/auth/session", SessionHandler)
	authenticated.PUT("/products/list", handlers.listProducts)
	authenticated.PUT("/products/:id/detail", handlers.getProductDetail)
	authenticated.PUT("/auth/logout", LogoutHandler)   // 注销当前登录会话。
	authenticated.PUT("/orders", handlers.createOrder) // 根据请求中的商品创建订单。
	authenticated.PUT("/orders/:id/pay", handlers.payOrder)
	authenticated.PUT("/orders/:id/receive", handlers.confirmReceipt)
	authenticated.PUT("/orders/list", handlers.listOrders) // 查询当前用户有权查看的订单。

	// 管理员路由：组内接口都必须登录并通过管理员权限校验。
	admin := router.Group("/api")
	admin.Use(RequireAuth(), RequireAdmin())
	admin.PUT("/admin/orders/:id/user", handlers.adminOrderUser)
	admin.PUT("/logs/query", LogQueryHandler(server.NewLogService(logger.LogDirectory())))
	admin.PUT("/products/export", handlers.exportProducts)    // 按筛选条件导出全部商品详情。
	admin.PUT("/products/image", handlers.uploadProductImage) // 上传商品图片并返回图片地址。
	admin.PUT("/products", handlers.createProduct)            // 创建商品。
	admin.PUT("/products/:id", handlers.updateProduct)        // 更新指定 ID 的商品。
	admin.PUT("/admin/orders/list", handlers.listAdminOrders)
	admin.PUT("/orders/:id/items/:item_id/ship", handlers.shipOrderItem)
	admin.PUT("/orders/:id/ship", handlers.shipOrder) // 将指定订单标记为已发货。
}
