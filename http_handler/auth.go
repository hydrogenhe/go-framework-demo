package http_handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go-framework-demo/http_handler/http_msg"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	redisclient "github.com/go-redis/redis/v8"
	appconst "go-framework-demo/const"
	"go-framework-demo/db"
	"go-framework-demo/model"
	"go-framework-demo/redis"
)

// AdminEmail 是配置的管理员账号邮箱。
const AdminEmail = "361861860@qq.com"

// IsAdmin 检查用户是否符合服务端配置的管理员身份。
func IsAdmin(user *model.User) bool {
	return user != nil && user.ID == 1 && user.Name == "hwt" && user.Email == AdminEmail
}
func forbiddenError() error {
	return &appconst.MsgError{Code: 40301, Message: "无权执行此操作", HTTPStatus: http.StatusForbidden}
}

// currentUser 获取已认证用户，缺少身份返回业务错误，类型异常返回内部错误。
func currentUser(c *gin.Context) (*model.User, error) {
	value, exists := c.Get("authenticatedUser")
	if !exists {
		return nil, appconst.NewUnauthorizedError("请先登录")
	}
	user, ok := value.(*model.User)
	if !ok {
		return nil, appconst.NewInternalError(fmt.Errorf("authenticatedUser 类型错误：%T", value))
	}
	if user == nil || user.ID <= 0 {
		return nil, appconst.NewUnauthorizedError("请先登录")
	}
	return user, nil
}

// validateUserIDAccess 核对请求中的用户 ID 是否属于服务端已认证的用户。
// 跨用户访问必须由接口明确允许，仅管理员查询可以放行。
func validateUserIDAccess(user *model.User, userID int64, allowAdmin bool) error {
	if user == nil || user.ID <= 0 {
		return appconst.NewUnauthorizedError("请先登录")
	}
	if userID <= 0 {
		return appconst.NewInvalidParamsError("用户ID必须为正整数")
	}
	if userID != user.ID && !(allowAdmin && IsAdmin(user)) {
		return forbiddenError()
	}
	return nil
}

// SessionKey 生成登录会话的 Redis 键，使用 token 哈希而非原始凭证。
func SessionKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return "auth:session:" + hex.EncodeToString(digest[:])
}
func authResponse(c *gin.Context, user *model.User, status int) {
	ctx := c.Request.Context()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		writeError(c, appconst.NewInternalError(err))
		return
	}
	token := hex.EncodeToString(raw)
	if err := redis.RedisClient.Set(ctx, SessionKey(token), strconv.FormatInt(user.ID, 10), 24*time.Hour).Err(); err != nil {
		writeError(c, appconst.NewInternalError(err))
		return
	}
	role := "user"
	if IsAdmin(user) {
		role = "admin"
	}
	response := http_msg.AuthResp{ID: strconv.FormatInt(user.ID, 10), Name: user.Name, Age: user.Age, Email: user.Email, CreateTime: user.CreateTime, UpdateTime: user.UpdateTime, Role: role, Token: token}
	writeSuccess(c, status, response)
}

// RequireAuth 校验 Bearer 凭证的服务端会话，并将已认证用户放入请求上下文。
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		header := c.GetHeader("Authorization")
		token := strings.TrimPrefix(header, "Bearer ")
		if !strings.HasPrefix(header, "Bearer ") || len(token) != 64 || redis.RedisClient == nil {
			writeError(c, appconst.NewUnauthorizedError("请先登录"))
			c.Abort()
			return
		}
		value, err := redis.RedisClient.Get(ctx, SessionKey(token)).Result()
		if errors.Is(err, redisclient.Nil) {
			writeError(c, appconst.NewUnauthorizedError("登录已过期，请重新登录"))
			c.Abort()
			return
		}
		if err != nil {
			writeError(c, appconst.NewInternalError(err))
			c.Abort()
			return
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			writeError(c, appconst.NewUnauthorizedError("登录无效"))
			c.Abort()
			return
		}
		userDB, err := db.NewUserDb(ctx)
		if err != nil {
			writeError(c, appconst.NewInternalError(err))
			c.Abort()
			return
		}
		user, err := userDB.GetByID(ctx, id)
		if err != nil {
			writeError(c, appconst.NewUnauthorizedError("账号不存在，请重新登录"))
			c.Abort()
			return
		}
		if user == nil || user.ID != id {
			writeError(c, appconst.NewUnauthorizedError("登录无效"))
			c.Abort()
			return
		}
		c.Set("authenticatedUser", user)
		c.Next()
	}
}

// RequireAdmin 拒绝已登录用户不具备管理员身份的请求。
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := currentUser(c)
		if err != nil {
			writeError(c, err)
			c.Abort()
			return
		}
		if !IsAdmin(user) {
			writeError(c, forbiddenError())
			c.Abort()
			return
		}
		c.Next()
	}
}

// LogoutHandler 删除当前 Bearer 凭证对应的服务端会话。
// 退出登录。
func LogoutHandler(c *gin.Context) {
	ctx := c.Request.Context()
	token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if err := redis.RedisClient.Del(ctx, SessionKey(token)).Err(); err != nil {
		writeError(c, appconst.NewInternalError(err))
		return
	}
	writeSuccess(c, http.StatusOK, http_msg.EmptyResp{})
}

// SessionHandler 确认当前会话有效，身份校验统一由 RequireAuth 完成。
func SessionHandler(c *gin.Context) {
	user, err := currentUser(c)
	if err != nil {
		writeError(c, err)
		return
	}
	role := "user"
	if IsAdmin(user) {
		role = "admin"
	}
	writeSuccess(c, http.StatusOK, gin.H{"id": strconv.FormatInt(user.ID, 10), "name": user.Name, "role": role})
}
