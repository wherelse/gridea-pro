package repository

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"sync"
	"time"

	"gridea-pro/backend/internal/domain"
	"gridea-pro/backend/internal/utils"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

type galleryRepository struct {
	mu     sync.RWMutex
	appDir string
	cache  []domain.GalleryPhoto
	sort   string
	loaded bool
}

func NewGalleryRepository(appDir string) domain.GalleryRepository {
	return &galleryRepository{
		appDir: appDir,
		cache:  make([]domain.GalleryPhoto, 0),
		sort:   domain.GallerySortManual,
		loaded: false,
	}
}

// galleryPhotoDTO JSON 序列化 DTO（时间兼容 string / int64；Description 兼容旧数据）
type galleryPhotoDTO struct {
	ID          string      `json:"id"`
	Image       string      `json:"image"`
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	Location    string      `json:"location"`
	TakenAt     interface{} `json:"takenAt"`
	CameraMake  string      `json:"cameraMake"`
	CameraModel string      `json:"cameraModel"`
	Lens        string      `json:"lens"`
	Aperture    string      `json:"aperture"`
	Shutter     string      `json:"shutter"`
	ISO         string      `json:"iso"`
	FocalLength string      `json:"focalLength"`
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	IsTop       bool        `json:"isTop"`
	CreatedAt   interface{} `json:"createdAt"`
	UpdatedAt   interface{} `json:"updatedAt"`
}

// parseTimeOrZero 与 parseTime 类似，但空值/无法识别时返回零值（用于拍摄时间）。
func parseTimeOrZero(v interface{}) time.Time {
	switch t := v.(type) {
	case string:
		if t == "" {
			return time.Time{}
		}
		if parsed, err := utils.ParseTime(t, time.Local); err == nil {
			return parsed
		}
		if t2, err := time.ParseInLocation(domain.TimeLayout, t, time.Local); err == nil {
			return t2
		}
		return time.Time{}
	case float64:
		return time.UnixMilli(int64(t))
	case int64:
		return time.UnixMilli(t)
	default:
		return time.Time{}
	}
}

func (r *galleryRepository) forceLoad() error {
	dbPath := filepath.Join(r.appDir, "config", "gallery.json")
	var db struct {
		Sort   string             `json:"sort"`
		Photos []galleryPhotoDTO `json:"photos"`
	}

	if err := LoadJSONFile(dbPath, &db); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			r.cache = []domain.GalleryPhoto{}
			r.sort = domain.GallerySortManual
			r.loaded = true
			return nil
		}
		log.Printf("[repo] galleryRepo load %s failed: %v", dbPath, err)
		return fmt.Errorf("load gallery: %w", err)
	}

	photos := make([]domain.GalleryPhoto, len(db.Photos))
	for i, p := range db.Photos {
		title := p.Title
		if title == "" {
			title = p.Description // 兼容旧的单字段 description
		}
		photos[i] = domain.GalleryPhoto{
			ID:          p.ID,
			Image:       p.Image,
			Title:       title,
			Location:    p.Location,
			TakenAt:     parseTimeOrZero(p.TakenAt),
			CameraMake:  p.CameraMake,
			CameraModel: p.CameraModel,
			Lens:        p.Lens,
			Aperture:    p.Aperture,
			Shutter:     p.Shutter,
			ISO:         p.ISO,
			FocalLength: p.FocalLength,
			Width:       p.Width,
			Height:      p.Height,
			IsTop:       p.IsTop,
			CreatedAt:   parseTime(p.CreatedAt),
			UpdatedAt:   parseTime(p.UpdatedAt),
		}
	}
	r.cache = photos
	r.sort = db.Sort
	if !domain.IsValidGallerySort(r.sort) {
		r.sort = domain.GallerySortManual
	}
	r.loaded = true
	return nil
}

func (r *galleryRepository) loadIfNeeded() error {
	r.mu.RLock()
	if r.loaded {
		r.mu.RUnlock()
		return nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loaded {
		return nil
	}
	return r.forceLoad()
}

// saveCache 写盘。调用方需持有 Lock。
func (r *galleryRepository) saveCache() error {
	dbPath := filepath.Join(r.appDir, "config", "gallery.json")

	dtos := make([]galleryPhotoDTO, len(r.cache))
	for i, p := range r.cache {
		takenAt := ""
		if !p.TakenAt.IsZero() {
			takenAt = p.TakenAt.Local().Format(time.RFC3339)
		}
		dtos[i] = galleryPhotoDTO{
			ID:          p.ID,
			Image:       p.Image,
			Title:       p.Title,
			Location:    p.Location,
			TakenAt:     takenAt,
			CameraMake:  p.CameraMake,
			CameraModel: p.CameraModel,
			Lens:        p.Lens,
			Aperture:    p.Aperture,
			Shutter:     p.Shutter,
			ISO:         p.ISO,
			FocalLength: p.FocalLength,
			Width:       p.Width,
			Height:      p.Height,
			IsTop:       p.IsTop,
			CreatedAt:   p.CreatedAt.Local().Format(time.RFC3339),
			UpdatedAt:   p.UpdatedAt.Local().Format(time.RFC3339),
		}
	}

	sortVal := r.sort
	if !domain.IsValidGallerySort(sortVal) {
		sortVal = domain.GallerySortManual
	}

	return SaveJSONFile(dbPath, map[string]interface{}{
		"sort":   sortVal,
		"photos": dtos,
	})
}

func ensureGalleryID(photo *domain.GalleryPhoto) {
	if photo.ID == "" {
		const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		id, _ := gonanoid.Generate(alphabet, 6)
		photo.ID = id
	}
}

// Invalidate 清除缓存标志，下次访问时从磁盘重新加载
func (r *galleryRepository) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loaded = false
}

func (r *galleryRepository) SaveAll(ctx context.Context, photos []domain.GalleryPhoto) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range photos {
		ensureGalleryID(&photos[i])
	}

	originalCache := r.cache
	originalLoaded := r.loaded
	r.cache = photos
	r.loaded = true

	if err := r.saveCache(); err != nil {
		r.cache = originalCache
		r.loaded = originalLoaded
		return err
	}
	return nil
}

func (r *galleryRepository) Create(ctx context.Context, photo *domain.GalleryPhoto) error {
	ensureGalleryID(photo)
	if err := r.loadIfNeeded(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	newCache := make([]domain.GalleryPhoto, len(r.cache)+1)
	copy(newCache, r.cache)
	newCache[len(r.cache)] = *photo

	originalCache := r.cache
	r.cache = newCache
	if err := r.saveCache(); err != nil {
		r.cache = originalCache
		return err
	}
	return nil
}

func (r *galleryRepository) Update(ctx context.Context, id string, photo *domain.GalleryPhoto) error {
	if err := r.loadIfNeeded(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	idx := -1
	for i, p := range r.cache {
		if p.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("photo not found")
	}

	newCache := make([]domain.GalleryPhoto, len(r.cache))
	copy(newCache, r.cache)
	newCache[idx] = *photo

	originalCache := r.cache
	r.cache = newCache
	if err := r.saveCache(); err != nil {
		r.cache = originalCache
		return err
	}
	return nil
}

func (r *galleryRepository) Delete(ctx context.Context, id string) error {
	if err := r.loadIfNeeded(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	idx := -1
	for i, p := range r.cache {
		if p.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("photo not found")
	}

	newCache := make([]domain.GalleryPhoto, 0, len(r.cache)-1)
	newCache = append(newCache, r.cache[:idx]...)
	newCache = append(newCache, r.cache[idx+1:]...)

	originalCache := r.cache
	r.cache = newCache
	if err := r.saveCache(); err != nil {
		r.cache = originalCache
		return err
	}
	return nil
}

func (r *galleryRepository) GetByID(ctx context.Context, id string) (*domain.GalleryPhoto, error) {
	if err := r.loadIfNeeded(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.cache {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("photo not found")
}

// List 按存储顺序（可用于手动排序）返回副本。
func (r *galleryRepository) List(ctx context.Context) ([]domain.GalleryPhoto, error) {
	if err := r.loadIfNeeded(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.GalleryPhoto, len(r.cache))
	copy(result, r.cache)
	return result, nil
}

func (r *galleryRepository) GetSort(ctx context.Context) (string, error) {
	if err := r.loadIfNeeded(); err != nil {
		return "", err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !domain.IsValidGallerySort(r.sort) {
		return domain.GallerySortManual, nil
	}
	return r.sort, nil
}

func (r *galleryRepository) SetSort(ctx context.Context, sort string) error {
	if err := r.loadIfNeeded(); err != nil {
		return err
	}
	if !domain.IsValidGallerySort(sort) {
		sort = domain.GallerySortManual
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	original := r.sort
	r.sort = sort
	if err := r.saveCache(); err != nil {
		r.sort = original
		return err
	}
	return nil
}
