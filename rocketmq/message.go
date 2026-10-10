package rocketmq

import (
	"encoding/json"
	"fmt"
	"go-framework-demo/util"
	"time"
)

// -------------------------- 通用枚举定义 --------------------------
// ActionType 操作类型枚举（统一管理所有业务的操作类型）
type ActionType string

// 通用操作类型常量（可根据业务扩展）
const (
	ActionCreate ActionType = "CREATE" // 新增
	ActionUpdate ActionType = "UPDATE" // 修改
	ActionDelete ActionType = "DELETE" // 删除
	ActionStatus ActionType = "STATUS" // 状态变更
	ActionQuery  ActionType = "QUERY"  // 查询（一般消息不建议用于查询，仅作示例）
)

// Validate 验证操作类型是否合法
func (a ActionType) Validate() error {
	switch a {
	case ActionCreate, ActionUpdate, ActionDelete, ActionStatus, ActionQuery:
		return nil
	default:
		return fmt.Errorf("不支持的操作类型：%s", a)
	}
}

// -------------------------- User业务消息结构体 --------------------------
// UserMessage 用户业务消息结构体
type UserMessage struct {
	Action    ActionType `json:"action"`    // 操作类型（新增/修改/删除）
	UserID    string     `json:"user_id"`   // 用户ID
	UserName  string     `json:"user_name"` // 用户名
	Phone     string     `json:"phone"`     // 手机号
	Email     string     `json:"email"`     // 邮箱
	ExtInfo   string     `json:"ext_info"`  // 扩展信息（按需添加）
	Timestamp int64      `json:"timestamp"` // 消息发送时间戳（毫秒）
}

// NewUserMessage 创建用户消息（构造函数，保证字段合法性）
func NewUserMessage(action ActionType, userID, userName string) (*UserMessage, error) {
	if err := action.Validate(); err != nil {
		return nil, err
	}
	if userID == "" {
		return nil, fmt.Errorf("用户ID不能为空")
	}
	return &UserMessage{
		Action:    action,
		UserID:    userID,
		UserName:  userName,
		Timestamp: time.Now().UnixMilli(), // 自动填充当前时间戳
	}, nil
}

// ToJSON 序列化为JSON字符串（消息发送时使用）
func (u *UserMessage) ToJSON() (string, error) {
	data, err := util.Marshal(u)
	if err != nil {
		return "", fmt.Errorf("UserMessage序列化失败：%w", err)
	}
	return string(data), nil
}

// ParseUserMessage 从JSON字符串反序列化（消费消息时使用）
func ParseUserMessage(jsonStr string) (*UserMessage, error) {
	var msg UserMessage
	if err := util.Unmarshal([]byte(jsonStr), &msg); err != nil {
		return nil, fmt.Errorf("UserMessage反序列化失败：%w", err)
	}
	if err := msg.Action.Validate(); err != nil {
		return nil, fmt.Errorf("操作类型不合法：%w", err)
	}
	return &msg, nil
}

// -------------------------- Order业务消息结构体 --------------------------
// OrderMessage 订单业务消息结构体
type OrderMessage struct {
	Action    ActionType `json:"action"`     // 操作类型
	OrderID   string     `json:"order_id"`   // 订单ID
	UserID    string     `json:"user_id"`    // 下单用户ID
	ProductID string     `json:"product_id"` // 商品ID
	Amount    float64    `json:"amount"`     // 订单金额
	Status    string     `json:"status"`     // 订单状态（待支付/已支付/已取消）
	Timestamp int64      `json:"timestamp"`  // 消息发送时间戳
}

// NewOrderMessage 创建订单消息（构造函数）
func NewOrderMessage(action ActionType, orderID, userID string, amount float64) (*OrderMessage, error) {
	if err := action.Validate(); err != nil {
		return nil, err
	}
	if orderID == "" || userID == "" {
		return nil, fmt.Errorf("订单ID和用户ID不能为空")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("订单金额必须大于0")
	}
	return &OrderMessage{
		Action:    action,
		OrderID:   orderID,
		UserID:    userID,
		Amount:    amount,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

// ToJSON 序列化为JSON字符串
func (o *OrderMessage) ToJSON() (string, error) {
	data, err := json.Marshal(o)
	if err != nil {
		return "", fmt.Errorf("OrderMessage序列化失败：%w", err)
	}
	return string(data), nil
}

// ParseOrderMessage 从JSON字符串反序列化
func ParseOrderMessage(jsonStr string) (*OrderMessage, error) {
	var msg OrderMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		return nil, fmt.Errorf("OrderMessage反序列化失败：%w", err)
	}
	if err := msg.Action.Validate(); err != nil {
		return nil, fmt.Errorf("操作类型不合法：%w", err)
	}
	return &msg, nil
}

// -------------------------- Product业务消息结构体 --------------------------
// ProductMessage 商品业务消息结构体
type ProductMessage struct {
	Action      ActionType `json:"action"`       // 操作类型
	ProductID   string     `json:"product_id"`   // 商品ID
	ProductName string     `json:"product_name"` // 商品名称
	Price       float64    `json:"price"`        // 商品价格
	Stock       int        `json:"stock"`        // 商品库存
	Timestamp   int64      `json:"timestamp"`    // 消息发送时间戳
}

// NewProductMessage 创建商品消息（构造函数）
func NewProductMessage(action ActionType, productID, productName string, price float64, stock int) (*ProductMessage, error) {
	if err := action.Validate(); err != nil {
		return nil, err
	}
	if productID == "" || productName == "" {
		return nil, fmt.Errorf("商品ID和名称不能为空")
	}
	if price <= 0 || stock < 0 {
		return nil, fmt.Errorf("商品价格必须大于0，库存不能为负数")
	}
	return &ProductMessage{
		Action:      action,
		ProductID:   productID,
		ProductName: productName,
		Price:       price,
		Stock:       stock,
		Timestamp:   time.Now().UnixMilli(),
	}, nil
}

// ToJSON 序列化为JSON字符串
func (p *ProductMessage) ToJSON() (string, error) {
	data, err := util.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("ProductMessage序列化失败：%w", err)
	}
	return string(data), nil
}

// ParseProductMessage 从JSON字符串反序列化
func (p *ProductMessage) ParseProductMessage(jsonStr string) (*ProductMessage, error) {
	var msg ProductMessage
	if err := util.Unmarshal([]byte(jsonStr), &msg); err != nil {
		return nil, fmt.Errorf("ProductMessage反序列化失败：%w", err)
	}
	if err := msg.Action.Validate(); err != nil {
		return nil, fmt.Errorf("操作类型不合法：%w", err)
	}
	return &msg, nil
}
