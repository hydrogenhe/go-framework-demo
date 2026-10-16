# 商品图片组件：SeaweedFS mini

SeaweedFS 4.48 作为独立 Docker 容器运行，采用 S3 上传、Filer HTTP 读取。商品只保留一张图片，MySQL `product.image` 和 ES 保存图片 URL，不保存图片二进制。

## 启动、重启与停止

在项目根目录执行：

```bash
./components_sh/restart_image.sh
```

脚本从 `conf/image_storage.yaml` 读取 S3 密钥和 Bucket，停止并删除旧容器，然后重新创建、检查服务就绪；不会清空 `data/`。本机与云端统一使用这一份配置。

```bash
./components_sh/restart_image.sh --stop
docker logs --tail 50 demo-seaweedfs
docker inspect demo-seaweedfs --format '{{.State.Status}}'
```

只操作图片组件，不启动或重启 Go 后端、MySQL、ES、MQ、Canal。

## 目录及地址

- `compose.yaml`：固定 SeaweedFS 4.48，容器名 `demo-seaweedfs`，自动重启策略 `unless-stopped`。
- `../../conf/image_storage.yaml`：Go 后端和本地 SeaweedFS 共同使用的 S3 endpoint、Bucket、访问密钥及 public_url。切换环境时直接修改这份配置。
- `data/`：对象数据和元数据持久化目录，不提交 Git；备份时需要保留整个目录。

| 服务 | 地址 |
|---|---|
| S3 API | http://localhost:8333 |
| 图片目录 / Filer | http://localhost:8888/buckets/product-images/products/ |
| Master | http://localhost:9333 |
| 管理界面 | http://localhost:23646 |

所有端口只绑定本机回环地址。Filer 和管理界面用于本地 Demo 管理；Filer 本身提供文件读写，不等同于经过管理员权限校验的业务上传接口。

图片最终位于 `component/Image_component/data/` 内的 SeaweedFS 存储文件中，不是一张图片对应一个普通文件；不要手动修改存储文件。

## 业务接入

1. 管理员在添加商品表单选择一张图片，可不选。
2. 前端 PUT `/api/products/image`，使用 multipart/form-data，文件字段为 `file`，携带登录 Bearer token。不要手动设置 Content-Type，由浏览器生成 multipart boundary。
3. HTTP Handler 限制请求大小、解析文件，调用 ImageService；Service 校验 JPEG/PNG、5MB、最多 4000 万像素，通过 S3 SDK 写入 `product-images/products/<随机UUID>.png` 或 `.jpg`。
4. 接口返回 `{ "image": "http://localhost:8888/buckets/product-images/products/<UUID>.png" }`。
5. 前端再创建商品，把该 URL 写入 `image` 字段，沿用 MySQL → Canal → MQ → ES 同步。商品创建成功后仍等待 1 秒刷新。

创建或更新商品本身不会重新上传图片；现有图片地址继续有效。上传成功而商品保存失败时会留下一张未引用图片，本 Demo 暂不自动清理。

Go 后端加载新代码后才能提供新上传接口，本组件脚本不负责加载后端代码。S3 endpoint 与面向浏览器的 public_url 是两个独立地址；如将来部署到服务器，需要相应修改配置与公开图片域名。

## 测试

测试调用实际 ImageService 和 HTTP Handler，使用 S3 HTTP 测试服务检查签名请求和图片字节，也可验证真实 SeaweedFS：

```bash
GOCACHE=/tmp/go-framework-demo-cache go test ./http_handler/test -run '^TestProductImage' -count=1 -v
RUN_IMAGE_INTEGRATION=1 GOCACHE=/tmp/go-framework-demo-cache go test ./http_handler/test -run '^TestSeaweedProductImageIntegration$' -count=1 -v
```

官方参考：[weed mini](https://github.com/seaweedfs/seaweedfs/wiki/Quick-Start-with-weed-mini)。
