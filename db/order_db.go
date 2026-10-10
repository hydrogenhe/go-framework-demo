package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/query"

	"gorm.io/gorm"
)

// TxContextKey 事务上下文key（和user表复用，无需重复定义）

// OrderDbInterface 订单表DB层接口（全程int64分单位，无float64）
type OrderDbInterface interface {
	Create(orderNo string, userId int64, amountFen int64) (*model.Order, error)
	GetByID(id int64) (*model.Order, error)
	GetByOrderNo(orderNo string) (*model.Order, error)
	GetByUserID(userId int64) ([]*model.Order, error)
	Update(id int64, updateData map[string]interface{}) (*model.Order, error)
	Delete(id int64) (bool, error)
	BatchCreateOrders(orders []*model.Order) error
	CreateWithItems(orderNo string, userID int64, items []*model.OrderItem) (*model.Order, error)
}

// CreateWithItems 在同一事务中创建订单和订单明细。
func (o *OrderDb) CreateWithItems(orderNo string, userID int64, items []*model.OrderItem) (*model.Order, error) {
	if orderNo == "" || userID <= 0 || len(items) == 0 {
		return nil, errors.New("新增订单失败：订单号、用户ID和订单明细不能为空")
	}
	var created *model.Order
	err := o.GenQuery.Transaction(func(txQ *query.Query) error {
		var amount int64
		for _, item := range items {
			if item == nil || item.ProductID <= 0 || item.Quantity <= 0 || item.UnitPriceFen <= 0 {
				return errors.New("新增订单失败：订单明细参数无效")
			}
			amount += int64(item.Quantity) * item.UnitPriceFen
		}
		if amount <= 0 {
			return errors.New("新增订单失败：订单金额必须大于0")
		}
		created = &model.Order{OrderNo: orderNo, UserID: userID, Amount: amount}
		if err := txQ.Order.WithContext(context.Background()).Create(created); err != nil {
			return fmt.Errorf("创建订单失败：%w", err)
		}
		itemDB, err := NewTxOrderItemDb(txQ)
		if err != nil {
			return err
		}
		for _, item := range items {
			item.OrderID = created.ID
			if _, err := itemDB.Create(item); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

type OrderDb struct {
	GenQuery *query.Query // Gen根查询器（绑定普通/事务DB）
}

// NewOrderDb 初始化OrderDb - 从上下文获取事务txQ，无则用全局Query
func NewOrderDb(ctx context.Context) (*OrderDb, error) {
	// 1. 从上下文取事务txQ
	var genQuery *query.Query
	if v := ctx.Value(TxContextKey{}); v != nil {
		genQuery = v.(*query.Query)
	}

	// 2. 无事务则用全局Query
	if genQuery == nil {
		genQuery = mysql.GetGenQuery()
	}

	if genQuery == nil || !genQuery.Available() {
		return nil, errors.New("Gen查询器实例为nil或未绑定有效DB")
	}

	return &OrderDb{
		GenQuery: genQuery,
	}, nil
}

// NewTxOrderDb 创建事务版OrderDb（供Gen事务回调使用）
func NewTxOrderDb(txQ *query.Query) (*OrderDb, error) {
	if txQ == nil || !txQ.Available() {
		return nil, errors.New("事务版Gen查询器无效")
	}
	return &OrderDb{
		GenQuery: txQ,
	}, nil
}

// --------------------------- 核心业务方法（全程int64分单位） ---------------------------

// GetByID 根据ID查询订单
func (o *OrderDb) GetByID(id int64) (*model.Order, error) {
	if id <= 0 {
		return nil, errors.New("查询失败：订单ID必须大于0")
	}

	orderDo := o.GenQuery.WithContext(context.Background()).Order
	order, err := orderDo.Where(o.GenQuery.Order.ID.Eq(id)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("查询失败：订单不存在（ID=%d）", id)
		}
		log.Printf("查询订单系统错误：ID=%d，错误=%v", id, err)
		return nil, fmt.Errorf("查询订单失败[系统错误]：%v（ID=%d）", err, id)
	}

	return order, nil
}

// GetByOrderNo 根据订单号查询订单（适配唯一键）
func (o *OrderDb) GetByOrderNo(orderNo string) (*model.Order, error) {
	if orderNo == "" {
		return nil, errors.New("查询失败：订单号不能为空")
	}

	orderDo := o.GenQuery.WithContext(context.Background()).Order
	order, err := orderDo.Where(o.GenQuery.Order.OrderNo.Eq(orderNo)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("查询失败：未找到匹配订单（订单号=%s）", orderNo)
		}
		log.Printf("查询订单系统错误：订单号=%s，错误=%v", orderNo, err)
		return nil, fmt.Errorf("查询订单失败[系统错误]：%v（订单号=%s）", err, orderNo)
	}

	return order, nil
}

// GetByUserID 根据用户ID查询订单列表（优化无数据场景）
func (o *OrderDb) GetByUserID(userId int64) ([]*model.Order, error) {
	if userId <= 0 {
		return nil, errors.New("查询失败：用户ID必须大于0")
	}

	orderDo := o.GenQuery.WithContext(context.Background()).Order
	orders, err := orderDo.Where(o.GenQuery.Order.UserID.Eq(userId)).Find()
	if err != nil {
		log.Printf("查询用户订单列表系统错误：用户ID=%d，错误=%v", userId, err)
		return nil, fmt.Errorf("查询用户订单列表失败[系统错误]：%v（用户ID=%d）", err, userId)
	}

	// 无数据返回空切片（语义精准，调用方无需判nil）
	if len(orders) == 0 {
		log.Printf("提示：用户ID=%d 未查询到订单数据", userId)
		return []*model.Order{}, nil
	}

	return orders, nil
}

// Update 更新订单（金额仅接收int64分单位）
func (o *OrderDb) Update(id int64, updateData map[string]interface{}) (*model.Order, error) {
	if id <= 0 {
		return nil, errors.New("更新失败：订单ID必须大于0")
	}
	if len(updateData) == 0 {
		return nil, errors.New("更新失败：请指定要修改的字段")
	}

	// 校验金额字段（仅接收int64分单位）
	if amountVal, ok := updateData["amount"]; ok {
		amountFen, isInt64 := amountVal.(int64)
		if !isInt64 {
			return nil, errors.New("更新失败：金额必须传入分单位的整数（int64类型）")
		}
		if amountFen <= 0 {
			return nil, errors.New("更新失败：订单金额（分）必须大于0")
		}
	}

	// 校验订单是否存在
	if _, err := o.GetByID(id); err != nil {
		return nil, err
	}

	orderDo := o.GenQuery.WithContext(context.Background()).Order
	resultInfo, err := orderDo.Where(o.GenQuery.Order.ID.Eq(id)).Updates(updateData)
	if err != nil {
		if isDuplicateOrderNoErr(err) {
			return nil, fmt.Errorf("更新失败：订单号已存在（违反唯一键约束）")
		}
		log.Printf("更新订单系统错误：ID=%d，错误=%v", id, err)
		return nil, fmt.Errorf("更新订单失败[系统错误]：%v（ID=%d）", err, id)
	}

	if resultInfo.RowsAffected == 0 {
		return nil, fmt.Errorf("更新失败：无数据被修改（ID=%d）", id)
	}

	// 返回更新后的订单
	return o.GetByID(id)
}

// Delete 删除订单
func (o *OrderDb) Delete(id int64) (bool, error) {
	if id <= 0 {
		return false, errors.New("删除失败：订单ID必须大于0")
	}

	// 校验订单是否存在
	if _, err := o.GetByID(id); err != nil {
		return false, err
	}

	orderDo := o.GenQuery.WithContext(context.Background()).Order
	_, err := orderDo.Where(o.GenQuery.Order.ID.Eq(id)).Delete()
	if err != nil {
		log.Printf("删除订单系统错误：ID=%d，错误=%v", id, err)
		return false, fmt.Errorf("删除订单失败[系统错误]：%v（ID=%d）", err, id)
	}

	return true, nil
}

// Create 新增订单（全程int64分单位，无float64）
func (o *OrderDb) Create(orderNo string, userId int64, amountFen int64) (*model.Order, error) {
	// 参数校验（金融级严格校验）
	if orderNo == "" {
		return nil, errors.New("新增失败：订单号不能为空")
	}
	if userId <= 0 {
		return nil, errors.New("新增失败：用户ID必须大于0")
	}
	if amountFen <= 0 {
		return nil, errors.New("新增失败：订单金额（分）必须大于0")
	}

	// 构建订单对象（直接赋值int64分，适配bigint字段）
	newOrder := &model.Order{
		OrderNo: orderNo,
		UserID:  userId,
		Amount:  amountFen, // 无类型转换，直接存分
	}

	orderDo := o.GenQuery.WithContext(context.Background()).Order
	if err := orderDo.Create(newOrder); err != nil {
		if isDuplicateOrderNoErr(err) {
			return nil, fmt.Errorf("新增失败：订单号已存在（%s）", orderNo)
		}
		log.Printf("新增订单系统错误：订单号=%s，错误=%v", orderNo, err)
		return nil, fmt.Errorf("新增订单失败[系统错误]：%v（订单信息：%+v）", err, newOrder)
	}

	return newOrder, nil
}

// BatchCreateOrders 批量新增订单（带事务，全程int64分）
func (o *OrderDb) BatchCreateOrders(orders []*model.Order) error {
	if len(orders) == 0 {
		return errors.New("批量新增失败：订单列表为空")
	}

	// 调用Gen内置事务
	err := o.GenQuery.Transaction(func(txQ *query.Query) error {
		txOrderDb, err := NewTxOrderDb(txQ)
		if err != nil {
			return fmt.Errorf("创建事务版OrderDb失败：%v", err)
		}

		// 事务内批量插入
		for idx, order := range orders {
			// 严格校验（分单位）
			if order.OrderNo == "" {
				return fmt.Errorf("第%d个订单无效：订单号不能为空", idx+1)
			}
			if order.UserID <= 0 {
				return fmt.Errorf("第%d个订单无效：用户ID必须大于0", idx+1)
			}
			if order.Amount <= 0 {
				return fmt.Errorf("第%d个订单无效：金额（分）必须大于0", idx+1)
			}

			orderDo := txOrderDb.GenQuery.WithContext(context.Background()).Order
			if err := orderDo.Create(order); err != nil {
				if isDuplicateOrderNoErr(err) {
					return fmt.Errorf("第%d个订单新增失败：订单号%s已存在", idx+1, order.OrderNo)
				}
				return fmt.Errorf("第%d个订单新增失败：%v（订单号=%s）", idx+1, err, order.OrderNo)
			}
		}
		return nil // 无错误则提交事务
	})

	if err != nil {
		return fmt.Errorf("批量新增订单失败：%v（事务已回滚）", err)
	}
	return nil
}

// --------------------------- 辅助方法 ---------------------------

// isDuplicateOrderNoErr 判断是否是订单号唯一键冲突错误
func isDuplicateOrderNoErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, gorm.ErrDuplicatedKey) ||
		strings.Contains(strings.ToLower(err.Error()), "duplicate entry") &&
			strings.Contains(strings.ToLower(err.Error()), "uk_order_no")
}
