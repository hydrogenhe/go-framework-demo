package http_handler

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	appconst "go-framework-demo/const"
	"go-framework-demo/http_handler/http_msg"
	"go-framework-demo/model"
	"go-framework-demo/server/product_server"
	"net/http"
	"strconv"
)

// 查询商品列表。
func (h routeHandlers) listProducts(c *gin.Context) {
	ctx := c.Request.Context()
	input, err := parseProductQuery(c, true)
	if err != nil {
		writeError(c, appconst.NewInvalidParamsError(err.Error()))
		return
	}
	v, e := h.productService.List(ctx, input)
	if e != nil {
		writeError(c, e)
		return
	}
	writeSuccess(c, 200, productPageResp(v))
}

// 查询商品详情，Service 按缓存优先、数据库回源的顺序读取。
func (h routeHandlers) getProductDetail(c *gin.Context) {
	ctx := c.Request.Context()
	var path http_msg.ProductPathReq
	if err := c.ShouldBindUri(&path); err != nil {
		writeError(c, appconst.NewInvalidParamsError("商品ID不能为空"))
		return
	}
	id, err := strconv.ParseInt(path.ID, 10, 64)
	if err != nil || id <= 0 {
		writeError(c, appconst.NewInvalidParamsError("商品ID必须为正整数"))
		return
	}
	product, err := h.productService.GetDetail(ctx, id)
	if err != nil {
		writeError(c, err)
		return
	}
	writeSuccess(c, http.StatusOK, productResp(product))
}

// CreateProductHandler 返回创建商品接口的 HTTP 处理函数，供测试或其他调用方复用。
func CreateProductHandler(s *product_server.ProductService) gin.HandlerFunc {
	return (routeHandlers{productService: s}).createProduct
}

// 创建商品。
func (h routeHandlers) createProduct(c *gin.Context) {
	ctx := c.Request.Context()
	var r http_msg.ProductCreateReq
	if c.ShouldBindJSON(&r) != nil {
		writeError(c, appconst.NewInvalidParamsError("请求参数格式不正确"))
		return
	}
	product := &model.Product{
		Name:  r.Name,
		Price: r.Price,
		Stock: r.Stock,
		Image: r.Image,
	}
	v, e := h.productService.CreateModel(ctx, product)
	if e != nil {
		writeError(c, e)
		return
	}
	writeSuccess(c, http.StatusCreated, productResp(v))
}

// 修改商品。
func (h routeHandlers) updateProduct(c *gin.Context) {
	ctx := c.Request.Context()
	var path http_msg.ProductPathReq
	if err := c.ShouldBindUri(&path); err != nil {
		writeError(c, appconst.NewInvalidParamsError("商品ID不能为空"))
		return
	}
	id, err := strconv.ParseInt(path.ID, 10, 64)
	if err != nil || id <= 0 {
		writeError(c, appconst.NewInvalidParamsError("商品ID必须为正整数"))
		return
	}
	var req http_msg.ProductUpdateReq
	decoder := json.NewDecoder(c.Request.Body)
	// 拒绝结构体未定义的 JSON 字段，避免拼错字段名被静默忽略，或提交禁止修改的字段。
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil {
		writeError(c, appconst.NewInvalidParamsError("请求参数格式不正确"))
		return
	}
	input := &product_server.ProductUpdateInput{
		Name:  req.Name,
		Price: req.Price,
		Stock: req.Stock,
		Image: req.Image,
	}
	v, e := h.productService.Update(ctx, id, input)
	if e != nil {
		writeError(c, e)
		return
	}
	writeSuccess(c, 200, productResp(v))
}
