package model

import "gorm.io/plugin/soft_delete"

// DeletedAt adapts GORM Gen's generated field type to the soft-delete plugin.
// Generated model files refer to this alias and remain untouched by hand.
type DeletedAt = soft_delete.DeletedAt
