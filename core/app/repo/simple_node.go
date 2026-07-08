package repo

import (
	"context"

	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/global"
)

type SimpleNodeRepo struct{}

type ISimpleNodeRepo interface {
	List(opts ...global.DBOption) ([]model.SimpleNode, error)
	Get(opts ...global.DBOption) (model.SimpleNode, error)
	Create(ctx context.Context, node *model.SimpleNode) error
	Save(ctx context.Context, node *model.SimpleNode) error
	Delete(ctx context.Context, opts ...global.DBOption) error
}

func NewISimpleNodeRepo() ISimpleNodeRepo {
	return &SimpleNodeRepo{}
}

func (r *SimpleNodeRepo) List(opts ...global.DBOption) ([]model.SimpleNode, error) {
	var items []model.SimpleNode
	db := global.DB.Model(&model.SimpleNode{})
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.Order("id desc").Find(&items).Error
	return items, err
}

func (r *SimpleNodeRepo) Get(opts ...global.DBOption) (model.SimpleNode, error) {
	var item model.SimpleNode
	db := global.DB.Model(&model.SimpleNode{})
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.First(&item).Error
	return item, err
}

func (r *SimpleNodeRepo) Create(ctx context.Context, node *model.SimpleNode) error {
	return global.DB.Create(node).Error
}

func (r *SimpleNodeRepo) Save(ctx context.Context, node *model.SimpleNode) error {
	return global.DB.Save(node).Error
}

func (r *SimpleNodeRepo) Delete(ctx context.Context, opts ...global.DBOption) error {
	db := global.DB.Model(&model.SimpleNode{})
	for _, opt := range opts {
		db = opt(db)
	}
	return db.Delete(&model.SimpleNode{}).Error
}
