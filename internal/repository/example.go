package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/arixbit/ginblade/internal/model"
)

// ExampleRepository persists examples.
type ExampleRepository struct {
	db *gorm.DB
}

// NewExampleRepository creates an ExampleRepository.
func NewExampleRepository(db *gorm.DB) *ExampleRepository {
	return &ExampleRepository{db: db}
}

// Create stores an example.
func (r *ExampleRepository) Create(ctx context.Context, example *model.Example) error {
	return dbFromContext(ctx, r.db).WithContext(ctx).Create(example).Error
}

// List returns examples ordered by newest first plus the total row count.
func (r *ExampleRepository) List(ctx context.Context, limit, offset int) ([]model.Example, int64, error) {
	db := dbFromContext(ctx, r.db).WithContext(ctx)

	var total int64
	if err := db.Model(&model.Example{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var examples []model.Example
	if err := db.Order("id DESC").Limit(limit).Offset(offset).Find(&examples).Error; err != nil {
		return nil, 0, err
	}

	return examples, total, nil
}
