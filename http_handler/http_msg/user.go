package http_msg

import "time"

type RegisterReq struct {
	Name     string `json:"name"`
	Age      int32  `json:"age"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
type LoginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResp 明确允许对外返回的字段，不包含密码及删除标志。
type AuthResp struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Age        int32     `json:"age"`
	Email      string    `json:"email"`
	CreateTime time.Time `json:"create_time"`
	UpdateTime time.Time `json:"update_time"`
	Role       string    `json:"role"`
	Token      string    `json:"token"`
}

// OrderUserDetailResp 订单页的用户资料，不含凭证或密码。
type OrderUserDetailResp struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
