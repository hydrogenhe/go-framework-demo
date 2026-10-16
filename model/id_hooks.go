package model

import (
	"go-framework-demo/util"
	"gorm.io/gorm"
	"time"
)

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.CreateTime.IsZero() {
		u.CreateTime = time.Now().Truncate(time.Millisecond)
	}
	if u.UpdateTime.IsZero() {
		u.UpdateTime = time.Now().Truncate(time.Millisecond)
	}
	if u.ID == 0 {
		u.ID = util.GetID()
	}
	return nil
}

func (u *User) BeforeUpdate(tx *gorm.DB) error {
	tx.Statement.SetColumn("UpdateTime", time.Now().Truncate(time.Millisecond))
	return nil
}
func (o *Order) BeforeCreate(tx *gorm.DB) error {
	if o.CreateTime.IsZero() {
		o.CreateTime = time.Now().Truncate(time.Millisecond)
	}
	if o.ExpireTime.IsZero() {
		o.ExpireTime = o.CreateTime.Add(OrderPaymentWindow)
	}
	if o.ID == 0 {
		o.ID = util.GetID()
	}
	if o.UpdateTime.IsZero() {
		o.UpdateTime = time.Now().Truncate(time.Millisecond)
	}
	return nil
}

func (o *Order) BeforeUpdate(tx *gorm.DB) error {
	tx.Statement.SetColumn("UpdateTime", time.Now().Truncate(time.Millisecond))
	return nil
}
func (p *Product) BeforeCreate(tx *gorm.DB) error {
	if p.CreateTime.IsZero() {
		p.CreateTime = time.Now()
	}
	if p.ID == 0 {
		p.ID = util.GetID()
	}
	if p.UpdateTime.IsZero() {
		p.UpdateTime = time.Now().Truncate(time.Millisecond)
	}
	return nil
}

func (p *Product) BeforeUpdate(tx *gorm.DB) error {
	tx.Statement.SetColumn("UpdateTime", time.Now().Truncate(time.Millisecond))
	return nil
}

func (i *OrderItem) BeforeCreate(tx *gorm.DB) error {
	if i.UpdateTime.IsZero() {
		i.UpdateTime = time.Now().Truncate(time.Millisecond)
	}
	if i.ID == 0 {
		i.ID = util.GetID()
	}
	return nil
}

func (i *OrderItem) BeforeUpdate(tx *gorm.DB) error {
	tx.Statement.SetColumn("UpdateTime", time.Now().Truncate(time.Millisecond))
	return nil
}
