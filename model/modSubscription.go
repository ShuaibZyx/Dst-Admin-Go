package model

import "gorm.io/gorm"

// ModSubscription 我的订阅（Steam Workshop 模组资产库，跨存档）
type ModSubscription struct {
	gorm.Model
	Modid       string `gorm:"uniqueIndex"`
	Name        string
	Img         string
	TimeUpdated int64
	SyncedAt    int64
}
