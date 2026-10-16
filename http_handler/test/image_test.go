package test

import (
	"bytes"
	"context"
	"encoding/json"
	"go-framework-demo/http_handler/http_msg"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	handler "go-framework-demo/http_handler"
	"go-framework-demo/model"
	"go-framework-demo/server/product_server"
	"go-framework-demo/storage"
)

// productImagePNG 生成一张小型 PNG 图片，作为上传和读取测试的数据。
func productImagePNG(t *testing.T) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	picture.Set(0, 0, color.RGBA{R: 255, A: 255})
	var data bytes.Buffer
	if err := png.Encode(&data, picture); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

// imageUploadRequest 构造包含图片文件的 multipart 请求，模拟前端上传。
func imageUploadRequest(t *testing.T, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "../../client-name.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/products/image", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

// TestProductImageUploadHandler 使用对象存储替身，验证图片上传的签名、随机文件名、原始内容和返回地址。
func TestProductImageUploadHandler(t *testing.T) {
	original := productImagePNG(t)
	var objectPath string
	calls := 0
	objectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPut || !strings.HasPrefix(r.URL.Path, "/product-images/products/") || !strings.HasSuffix(r.URL.Path, ".png") {
			t.Errorf("invalid object request: %s %s", r.Method, r.URL.Path)
		}
		if strings.Contains(r.URL.Path, "client-name") || !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256") || r.Header.Get("Content-Type") != "image/png" {
			t.Errorf("invalid upload headers or filename")
		}
		var body io.Reader = r.Body
		if strings.HasPrefix(r.Header.Get("X-Amz-Content-Sha256"), "STREAMING-") {
			body = httputil.NewChunkedReader(r.Body)
		}
		actual, err := io.ReadAll(body)
		if err != nil || !bytes.Equal(actual, original) {
			t.Errorf("image bytes changed: %v", err)
		}
		objectPath = r.URL.Path
		w.Header().Set("ETag", `"test"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer objectServer.Close()
	config := storage.ImageConfig{
		Endpoint:  objectServer.URL,
		Bucket:    "product-images",
		PublicURL: objectServer.URL + "/product-images",
		AccessKey: "test-access",
		SecretKey: "test-secret",
	}
	store, err := storage.NewImageStore(&config)
	if err != nil {
		t.Fatal(err)
	}
	previousStore := storage.ImageStoreClient
	storage.ImageStoreClient = store
	t.Cleanup(func() { storage.ImageStoreClient = previousStore })
	service := product_server.NewImageService()
	router := testRouter(t)
	admin := &model.User{
		ID:    1,
		Name:  "hwt",
		Email: handler.AdminEmail,
	}
	router.PUT("/api/products/image", func(c *gin.Context) { c.Set("authenticatedUser", admin) }, handler.RequireAdmin(), handler.UploadProductImageHandler(service))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, imageUploadRequest(t, original))
	if response.Code != http.StatusCreated {
		t.Fatalf("upload failed: %d %s", response.Code, response.Body.String())
	}
	var envelope http_msg.Response[http_msg.ProductImageResp]
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	result := envelope.Data
	if envelope.Code != 0 {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if calls != 1 || result.Image != objectServer.URL+objectPath {
		t.Fatalf("unexpected image response: %+v calls=%d", result, calls)
	}
}

// TestProductImageRejectsInvalidFiles 验证空文件、非图片文件和超出大小限制的文件均被拒绝。
func TestProductImageRejectsInvalidFiles(t *testing.T) {
	service := product_server.NewImageService()
	for _, input := range []struct {
		Name string
		Body []byte
	}{
		{Name: "empty"},
		{Name: "text", Body: []byte("not an image")},
		{Name: "oversize", Body: make([]byte, product_server.MaxProductImageSize+1)},
	} {
		t.Run(input.Name, func(t *testing.T) {
			router := testRouter(t)
			router.PUT("/api/products/image", handler.UploadProductImageHandler(service))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, imageUploadRequest(t, input.Body))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid image accepted: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

// TestProductImageRequiresAdmin 验证未登录用户及普通用户不能上传商品图片。
func TestProductImageRequiresAdmin(t *testing.T) {
	router := testRouter(t)
	handler.RegisterRoutes(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, imageUploadRequest(t, productImagePNG(t)))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated upload: %d", response.Code)
	}
	user := &model.User{ID: 2}
	protected := testRouter(t)
	protected.PUT("/api/products/image", func(c *gin.Context) { c.Set("authenticatedUser", user) }, handler.RequireAdmin(), handler.UploadProductImageHandler(product_server.NewImageService()))
	response = httptest.NewRecorder()
	protected.ServeHTTP(response, imageUploadRequest(t, productImagePNG(t)))
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-admin upload: %d", response.Code)
	}
}

// TestSeaweedProductImageIntegration 使用真实 SeaweedFS，验证上传或读取已有测试图片后内容一致。
func TestSeaweedProductImageIntegration(t *testing.T) {
	if os.Getenv("RUN_IMAGE_INTEGRATION") != "1" {
		t.Skip("set RUN_IMAGE_INTEGRATION=1 to use local SeaweedFS")
	}
	config, err := storage.LoadImageConfig()
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.NewImageStore(&config)
	if err != nil {
		t.Fatal(err)
	}
	service := product_server.NewImageService(store)
	original := productImagePNG(t)
	imageURL := os.Getenv("IMAGE_EXISTING_URL")
	if imageURL == "" {
		imageURL, err = service.Upload(context.Background(), bytes.NewReader(original))
		if err != nil {
			t.Fatal(err)
		}
	}
	response, err := http.Get(imageURL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	actual, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || !bytes.Equal(original, actual) {
		t.Fatalf("image read failed: status=%d err=%v body=%s", response.StatusCode, err, actual)
	}
	t.Logf("IMAGE_URL=%s", imageURL)
}
