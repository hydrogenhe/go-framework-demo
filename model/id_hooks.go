package model

import (
	"go-framework-demo/util"
	"gorm.io/gorm"
)

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == 0 {
		u.ID = util.GetID()
	}
	return nil
}
func (o *Order) BeforeCreate(tx *gorm.DB) error {
	if o.ID == 0 {
		o.ID = util.GetID()
	}
	return nil
}
func (p *Product) BeforeCreate(tx *gorm.DB) error {
	if p.ID == 0 {
		p.ID = util.GetID()
	}
	return nil
}

func (i *OrderItem) BeforeCreate(tx *gorm.DB) error {
	if i.ID == 0 {
		i.ID = util.GetID()
	}
	return nil
}
