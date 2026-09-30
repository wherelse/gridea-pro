package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// 相册排序方式
const (
	GallerySortManual   = "manual"    // 手动（存储/拖动顺序）
	GallerySortTimeDesc = "time-desc" // 按拍摄时间 新 → 旧
	GallerySortTimeAsc  = "time-asc"  // 按拍摄时间 旧 → 新
)

// IsValidGallerySort 校验排序方式
func IsValidGallerySort(s string) bool {
	switch s {
	case GallerySortManual, GallerySortTimeDesc, GallerySortTimeAsc:
		return true
	}
	return false
}

// GalleryPhoto 相册照片实体。
// Image 为站点相对路径（如 /images/gallery/xxx.webp）或外链 URL。
// Title/Location 为结构化文字字段；其余为从 EXIF 自动提取的信息。
type GalleryPhoto struct {
	ID       string `json:"id"`
	Image    string `json:"image"`
	Title    string `json:"title"`
	Location string `json:"location"`

	// EXIF
	TakenAt     time.Time `json:"takenAt" ts_type:"string"` // 拍摄时间
	CameraMake  string    `json:"cameraMake"`
	CameraModel string    `json:"cameraModel"`
	Lens        string    `json:"lens"`
	Aperture    string    `json:"aperture"`    // 如 f/2.8
	Shutter     string    `json:"shutter"`     // 如 1/250s
	ISO         string    `json:"iso"`         // 如 ISO 100
	FocalLength string    `json:"focalLength"` // 如 35mm
	Width       int       `json:"width"`
	Height      int       `json:"height"`

	IsTop     bool      `json:"isTop"` // 封面/置顶
	CreatedAt time.Time `json:"createdAt" ts_type:"string"`
	UpdatedAt time.Time `json:"updatedAt" ts_type:"string"`
}

// Validate 校验照片数据
func (p *GalleryPhoto) Validate() error {
	if strings.TrimSpace(p.Image) == "" {
		return errors.New("image is required")
	}
	return nil
}

// GalleryData 相册数据（排序方式 + 照片列表）
type GalleryData struct {
	Sort   string         `json:"sort"`
	Photos []GalleryPhoto `json:"photos"`
}

// GalleryRepository 定义相册存储接口
type GalleryRepository interface {
	Create(ctx context.Context, photo *GalleryPhoto) error
	Update(ctx context.Context, id string, photo *GalleryPhoto) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*GalleryPhoto, error)
	List(ctx context.Context) ([]GalleryPhoto, error)
	SaveAll(ctx context.Context, photos []GalleryPhoto) error
	// GetSort 返回输出排序方式（默认 manual）
	GetSort(ctx context.Context) (string, error)
	// SetSort 设置输出排序方式
	SetSort(ctx context.Context, sort string) error
}
