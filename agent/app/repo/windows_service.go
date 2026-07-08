package repo

import (
	"context"

	"github.com/1Panel-dev/1Panel/agent/app/model"
)

type WindowsServiceRepo struct{}

type IWindowsServiceRepo interface {
	List(opts ...DBOption) ([]model.WindowsService, error)
	Get(opts ...DBOption) (model.WindowsService, error)
	Create(ctx context.Context, service *model.WindowsService) error
	Save(ctx context.Context, service *model.WindowsService) error
	Delete(ctx context.Context, opts ...DBOption) error
}

func NewIWindowsServiceRepo() IWindowsServiceRepo {
	return &WindowsServiceRepo{}
}

func (r *WindowsServiceRepo) List(opts ...DBOption) ([]model.WindowsService, error) {
	var items []model.WindowsService
	db := getTx(context.Background()).Model(&model.WindowsService{})
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.Order("id desc").Find(&items).Error
	return items, err
}

func (r *WindowsServiceRepo) Get(opts ...DBOption) (model.WindowsService, error) {
	var item model.WindowsService
	db := getTx(context.Background()).Model(&model.WindowsService{})
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.First(&item).Error
	return item, err
}

func (r *WindowsServiceRepo) Create(ctx context.Context, service *model.WindowsService) error {
	return getTx(ctx).Create(service).Error
}

func (r *WindowsServiceRepo) Save(ctx context.Context, service *model.WindowsService) error {
	return getTx(ctx).Save(service).Error
}

func (r *WindowsServiceRepo) Delete(ctx context.Context, opts ...DBOption) error {
	db := getTx(ctx).Model(&model.WindowsService{})
	for _, opt := range opts {
		db = opt(db)
	}
	return db.Delete(&model.WindowsService{}).Error
}
