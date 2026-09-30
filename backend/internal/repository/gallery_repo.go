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

	gonanoid "github.com/matoous/go-nanoid/v2"
)

type galleryRepository struct {
	mu     sync.RWMutex
	appDir string
	cache  []domain.GalleryPhoto
	loaded bool
}

func NewGalleryRepository(appDir string) domain.GalleryRepository {
	return &galleryRepository{
		appDir: appDir,
		cache:  make([]domain.GalleryPhoto, 0),
		loaded: false,
	}
}

// galleryPhotoDTO JSON 序列化 DTO（时间兼容 string / int64）
type galleryPhotoDTO struct {
	ID          string      `json:"id"`
	Image       string      `json:"image"`
	Description string      `json:"description"`
	IsTop       bool        `json:"isTop"`
	CreatedAt   interface{} `json:"createdAt"`
	UpdatedAt   interface{} `json:"updatedAt"`
}

func (r *galleryRepository) forceLoad() error {
	dbPath := filepath.Join(r.appDir, "config", "gallery.json")
	var db struct {
		Photos []galleryPhotoDTO `json:"photos"`
	}

	if err := LoadJSONFile(dbPath, &db); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			r.cache = []domain.GalleryPhoto{}
			r.loaded = true
			return nil
		}
		log.Printf("[repo] galleryRepo load %s failed: %v", dbPath, err)
		return fmt.Errorf("load gallery: %w", err)
	}

	photos := make([]domain.GalleryPhoto, len(db.Photos))
	for i, p := range db.Photos {
		photos[i] = domain.GalleryPhoto{
			ID:          p.ID,
			Image:       p.Image,
			Description: p.Description,
			IsTop:       p.IsTop,
			CreatedAt:   parseTime(p.CreatedAt),
			UpdatedAt:   parseTime(p.UpdatedAt),
		}
	}
	r.cache = photos
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
		dtos[i] = galleryPhotoDTO{
			ID:          p.ID,
			Image:       p.Image,
			Description: p.Description,
			IsTop:       p.IsTop,
			CreatedAt:   p.CreatedAt.Local().Format(time.RFC3339),
			UpdatedAt:   p.UpdatedAt.Local().Format(time.RFC3339),
		}
	}

	return SaveJSONFile(dbPath, map[string]interface{}{"photos": dtos})
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

// List 按存储顺序（用户可拖动排序）返回副本。
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
