package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// GalleryPhoto 相册照片实体。
// Image 为站点相对路径（如 /images/gallery/xxx.webp）或外链 URL。
type GalleryPhoto struct {
	ID          string    `json:"id"`
	Image       string    `json:"image"`
	Description string    `json:"description"`
	IsTop       bool      `json:"isTop"` // 封面/置顶
	CreatedAt   time.Time `json:"createdAt" ts_type:"string"`
	UpdatedAt   time.Time `json:"updatedAt" ts_type:"string"`
}

// Validate 校验照片数据
func (p *GalleryPhoto) Validate() error {
	if strings.TrimSpace(p.Image) == "" {
		return errors.New("image is required")
	}
	return nil
}

// GalleryRepository 定义相册存储接口
type GalleryRepository interface {
	Create(ctx context.Context, photo *GalleryPhoto) error
	Update(ctx context.Context, id string, photo *GalleryPhoto) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*GalleryPhoto, error)
	List(ctx context.Context) ([]GalleryPhoto, error)
	SaveAll(ctx context.Context, photos []GalleryPhoto) error
}
