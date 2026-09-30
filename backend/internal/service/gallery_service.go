package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gridea-pro/backend/internal/domain"
	"gridea-pro/backend/internal/webpconvert"

	securejoin "github.com/cyphar/filepath-securejoin"
)

// GalleryService 相册业务逻辑：照片增删改查 + 图片落盘（统一压缩为 WebP）。
type GalleryService struct {
	repo   domain.GalleryRepository
	appDir string
	mu     sync.RWMutex
}

func NewGalleryService(repo domain.GalleryRepository, appDir string) *GalleryService {
	return &GalleryService{repo: repo, appDir: appDir}
}

// galleryDir 相册图片存放目录：站点 images/gallery/
func (s *GalleryService) galleryDir() string {
	return filepath.Join(s.appDir, "images", "gallery")
}

func (s *GalleryService) LoadPhotos(ctx context.Context) ([]domain.GalleryPhoto, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.repo.List(ctx)
}

// AddPhoto 新增照片记录（置顶到列表最前）。
func (s *GalleryService) AddPhoto(ctx context.Context, image, description string) (*domain.GalleryPhoto, error) {
	if strings.TrimSpace(image) == "" {
		return nil, fmt.Errorf("image is empty")
	}
	now := time.Now()
	photo := domain.GalleryPhoto{
		Image:       image,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	photos, err := s.repo.List(ctx)
	if err != nil {
		photos = []domain.GalleryPhoto{}
	}
	photos = append([]domain.GalleryPhoto{photo}, photos...)
	if err := s.repo.SaveAll(ctx, photos); err != nil {
		return nil, err
	}
	return &photos[0], nil
}

// UpdatePhoto 更新描述 / 图片 / 置顶标记。
func (s *GalleryService) UpdatePhoto(ctx context.Context, photo domain.GalleryPhoto) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	photos, err := s.repo.List(ctx)
	if err != nil {
		return err
	}

	found := false
	for i := range photos {
		if photos[i].ID == photo.ID {
			if strings.TrimSpace(photo.Description) != "" || photo.Description == "" {
				photos[i].Description = photo.Description
			}
			if strings.TrimSpace(photo.Image) != "" {
				photos[i].Image = photo.Image
			}
			photos[i].UpdatedAt = time.Now()
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("photo not found")
	}
	return s.repo.SaveAll(ctx, photos)
}

// DeletePhoto 删除照片记录，并清理其本地图片文件（仅限站点 images/gallery 内）。
func (s *GalleryService) DeletePhoto(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	photos, err := s.repo.List(ctx)
	if err != nil {
		return err
	}

	filtered := make([]domain.GalleryPhoto, 0, len(photos))
	found := false
	for _, p := range photos {
		if p.ID == id {
			found = true
			s.removeLocalImage(p.Image)
			continue
		}
		filtered = append(filtered, p)
	}
	if !found {
		return fmt.Errorf("photo not found")
	}
	return s.repo.SaveAll(ctx, filtered)
}

// removeLocalImage 删除站点内 images/gallery 下的图片文件；外链或越界路径不动。
func (s *GalleryService) removeLocalImage(image string) {
	image = strings.TrimSpace(image)
	if !strings.HasPrefix(image, "/images/gallery/") {
		return
	}
	rel := strings.TrimPrefix(image, "/")
	joined, err := securejoin.SecureJoin(s.appDir, rel)
	if err != nil {
		return
	}
	_ = os.Remove(joined)
}

// ReorderPhotos 按给定 ID 顺序重排。
func (s *GalleryService) ReorderPhotos(ctx context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	photos, err := s.repo.List(ctx)
	if err != nil {
		return err
	}

	byID := make(map[string]domain.GalleryPhoto, len(photos))
	for _, p := range photos {
		byID[p.ID] = p
	}

	reordered := make([]domain.GalleryPhoto, 0, len(photos))
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			reordered = append(reordered, p)
			delete(byID, id)
		}
	}
	// 未出现在 ids 中的（防御）追加在末尾，保持原相对顺序
	for _, p := range photos {
		if _, ok := byID[p.ID]; ok {
			reordered = append(reordered, p)
		}
	}
	return s.repo.SaveAll(ctx, reordered)
}

// SetCover 设置封面（IsTop），同一时间仅一张。
func (s *GalleryService) SetCover(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	photos, err := s.repo.List(ctx)
	if err != nil {
		return err
	}

	found := false
	for i := range photos {
		if photos[i].ID == id {
			photos[i].IsTop = true
			found = true
		} else {
			photos[i].IsTop = false
		}
	}
	if !found {
		return fmt.Errorf("photo not found")
	}
	return s.repo.SaveAll(ctx, photos)
}

// SaveImageFile 把磁盘上的图片复制进相册目录（可转换格式自动转 WebP），返回站点相对路径。
func (s *GalleryService) SaveImageFile(ctx context.Context, sourcePath string) (string, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", sourcePath)
	}
	return s.saveImageFromFile(sourcePath)
}

// SaveImageBytes 保存前端粘贴/拖拽的图片字节（base64 解码后传入）。
func (s *GalleryService) SaveImageBytes(ctx context.Context, name string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty image data")
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		ext = ".jpg"
		if ct := http.DetectContentType(data); strings.HasPrefix(ct, "image/") {
			switch {
			case strings.Contains(ct, "png"):
				ext = ".png"
			case strings.Contains(ct, "gif"):
				ext = ".gif"
			case strings.Contains(ct, "webp"):
				ext = ".webp"
			}
		}
	}

	tmp, err := os.CreateTemp("", "gallery-*"+ext)
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return s.saveImageFromFile(tmpPath)
}

// saveImageFromFile 落盘并（在可转换时）压缩为 WebP。
func (s *GalleryService) saveImageFromFile(srcPath string) (string, error) {
	dir := s.galleryDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	base := fmt.Sprintf("%d", time.Now().UnixNano())

	if webpconvert.NeedsConversion(srcPath) {
		if tmp, err := webpconvert.ConvertToWebP(srcPath, 80); err == nil && tmp != "" {
			defer os.Remove(tmp)
			name := base + ".webp"
			if err := copyLocalFile(tmp, filepath.Join(dir, name)); err == nil {
				return "/images/gallery/" + name, nil
			}
		}
	}

	name := base + strings.ToLower(filepath.Ext(srcPath))
	if err := copyLocalFile(srcPath, filepath.Join(dir, name)); err != nil {
		return "", err
	}
	return "/images/gallery/" + name, nil
}

func copyLocalFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
