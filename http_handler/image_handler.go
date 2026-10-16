package http_handler

import (
	"go-framework-demo/http_handler/http_msg"
	"net/http"

	"github.com/gin-gonic/gin"
	appconst "go-framework-demo/const"
	"go-framework-demo/server/product_server"
)

func UploadProductImageHandler(service *product_server.ImageService) gin.HandlerFunc {
	return (routeHandlers{imageService: service}).uploadProductImage

}

// 上传商品图片。
func (h routeHandlers) uploadProductImage(c *gin.Context) {
	ctx := c.Request.Context()
	// 限制整个 multipart 请求的大小，包含请求头和其他表单字段。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, product_server.MaxProductImageSize+(1<<20))
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		writeError(c, appconst.NewInvalidParamsError("上传请求无效，图片不能超过 5MB"))
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	files := c.Request.MultipartForm.File["file"]
	totalFiles := 0
	for _, values := range c.Request.MultipartForm.File {
		totalFiles += len(values)
	}
	if len(files) != 1 || totalFiles != 1 {
		writeError(c, appconst.NewInvalidParamsError("请上传一张图片，字段名为 file"))
		return
	}
	req := http_msg.ProductImageReq{File: files[0]}
	file, err := req.File.Open()
	if err != nil {
		writeError(c, appconst.NewInternalError(err))
		return
	}
	defer file.Close()
	imageURL, err := h.imageService.Upload(ctx, file)
	if err != nil {
		writeError(c, err)
		return
	}
	response := http_msg.ProductImageResp{
		Image: imageURL,
	}
	writeSuccess(c, http.StatusCreated, response)
}
