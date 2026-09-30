package facade

import (
	"context"
	"encoding/base64"
	"sync/atomic"

	"gridea-pro/backend/internal/domain"
	"gridea-pro/backend/internal/service"
)

// GalleryFacade wraps GalleryService（相册模块，仿闪念 MemoFacade）。
type GalleryFacade struct {
	// internal 用原子读写保存：切站（UpdateAppDir）会热替换它。
	internal atomic.Pointer[service.GalleryService]
}

func NewGalleryFacade(s *service.GalleryService) *GalleryFacade {
	f := &GalleryFacade{}
	f.internal.Store(s)
	return f
}

func (f *GalleryFacade) svc() *service.GalleryService { return f.internal.Load() }

func galleryCtx() context.Context {
	if WailsContext == nil {
		return context.TODO()
	}
	return WailsContext
}

// LoadGalleryFromFrontend 返回全部照片（按存储顺序）。
func (f *GalleryFacade) LoadGalleryFromFrontend() ([]domain.GalleryPhoto, error) {
	return f.svc().LoadPhotos(galleryCtx())
}

// AddPhotoByFileFromFrontend 从文件对话框选择的本地图片新增照片（自动压缩为 WebP）。
func (f *GalleryFacade) AddPhotoByFileFromFrontend(sourcePath string, description string) ([]domain.GalleryPhoto, error) {
	ctx := galleryCtx()
	svc := f.svc()
	image, err := svc.SaveImageFile(ctx, sourcePath)
	if err != nil {
		return nil, err
	}
	if _, err := svc.AddPhoto(ctx, image, description); err != nil {
		return nil, err
	}
	return svc.LoadPhotos(ctx)
}

// AddPhotoByBytesFromFrontend 从粘贴/拖拽的图片字节新增照片（base64 传入）。
func (f *GalleryFacade) AddPhotoByBytesFromFrontend(name string, dataBase64 string, description string) ([]domain.GalleryPhoto, error) {
	ctx := galleryCtx()
	svc := f.svc()
	data, err := base64.StdEncoding.DecodeString(dataBase64)
	if err != nil {
		return nil, err
	}
	image, err := svc.SaveImageBytes(ctx, name, data)
	if err != nil {
		return nil, err
	}
	if _, err := svc.AddPhoto(ctx, image, description); err != nil {
		return nil, err
	}
	return svc.LoadPhotos(ctx)
}

// UpdatePhotoFromFrontend 更新描述等字段。
func (f *GalleryFacade) UpdatePhotoFromFrontend(photo domain.GalleryPhoto) ([]domain.GalleryPhoto, error) {
	ctx := galleryCtx()
	svc := f.svc()
	if err := svc.UpdatePhoto(ctx, photo); err != nil {
		return nil, err
	}
	return svc.LoadPhotos(ctx)
}

// DeletePhotoFromFrontend 删除照片（并清理本地图片文件）。
func (f *GalleryFacade) DeletePhotoFromFrontend(id string) ([]domain.GalleryPhoto, error) {
	ctx := galleryCtx()
	svc := f.svc()
	if err := svc.DeletePhoto(ctx, id); err != nil {
		return nil, err
	}
	return svc.LoadPhotos(ctx)
}

// ReorderPhotosFromFrontend 按给定 ID 顺序重排。
func (f *GalleryFacade) ReorderPhotosFromFrontend(ids []string) ([]domain.GalleryPhoto, error) {
	ctx := galleryCtx()
	svc := f.svc()
	if err := svc.ReorderPhotos(ctx, ids); err != nil {
		return nil, err
	}
	return svc.LoadPhotos(ctx)
}

// SetCoverFromFrontend 设置封面（IsTop）。
func (f *GalleryFacade) SetCoverFromFrontend(id string) ([]domain.GalleryPhoto, error) {
	ctx := galleryCtx()
	svc := f.svc()
	if err := svc.SetCover(ctx, id); err != nil {
		return nil, err
	}
	return svc.LoadPhotos(ctx)
}
