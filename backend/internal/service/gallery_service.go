package service

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gridea-pro/backend/internal/domain"
	"gridea-pro/backend/internal/webpconvert"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/rwcarlsen/goexif/exif"
)

// GalleryService 相册业务逻辑：照片增删改查 + 图片落盘（统一压缩为 WebP）+ EXIF 提取。
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

func (s *GalleryService) LoadData(ctx context.Context) (*domain.GalleryData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	photos, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	sort, err := s.repo.GetSort(ctx)
	if err != nil {
		sort = domain.GallerySortManual
	}
	return &domain.GalleryData{Sort: sort, Photos: photos}, nil
}

func (s *GalleryService) SetSort(ctx context.Context, sort string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repo.SetSort(ctx, sort)
}

// AddPhoto 新增照片（置顶到列表最前）。
func (s *GalleryService) AddPhoto(ctx context.Context, photo domain.GalleryPhoto) (*domain.GalleryPhoto, error) {
	if strings.TrimSpace(photo.Image) == "" {
		return nil, fmt.Errorf("image is empty")
	}
	now := time.Now()
	photo.CreatedAt = now
	photo.UpdatedAt = now

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

// UpdatePhoto 更新标题 / 地点 / 图片。
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
			photos[i].Title = photo.Title
			photos[i].Location = photo.Location
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

// ExtractMetadata 从图片文件读取 EXIF / 尺寸，返回仅填充了元信息（不含 ID/Image）的 GalleryPhoto。
func (s *GalleryService) ExtractMetadata(path string) domain.GalleryPhoto {
	var p domain.GalleryPhoto

	// 尺寸（用标准库解码，最可靠）
	if f, err := os.Open(path); err == nil {
		if cfg, _, err := image.DecodeConfig(f); err == nil {
			p.Width = cfg.Width
			p.Height = cfg.Height
		}
		_ = f.Close()
	}

	f, err := os.Open(path)
	if err != nil {
		return p
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return p
	}

	if t, err := x.DateTime(); err == nil {
		p.TakenAt = t
	}
	p.CameraMake = exifString(x, exif.Make)
	p.CameraModel = exifString(x, exif.Model)
	p.Lens = exifString(x, exif.LensModel)

	if tag, err := x.Get(exif.FNumber); err == nil {
		if r, err := tag.Rat(0); err == nil {
			v, _ := r.Float64()
			p.Aperture = fmt.Sprintf("f/%.1f", v)
		}
	}
	if tag, err := x.Get(exif.ExposureTime); err == nil {
		if r, err := tag.Rat(0); err == nil {
			v, _ := r.Float64()
			if v > 0 && v < 1 {
				p.Shutter = fmt.Sprintf("1/%ds", int(math.Round(1/v)))
			} else if v >= 1 {
				p.Shutter = fmt.Sprintf("%ds", int(math.Round(v)))
			}
		}
	}
	if tag, err := x.Get(exif.ISOSpeedRatings); err == nil {
		if iso, err := tag.Int(0); err == nil {
			p.ISO = fmt.Sprintf("ISO %d", iso)
		}
	}
	if tag, err := x.Get(exif.FocalLength); err == nil {
		if r, err := tag.Rat(0); err == nil {
			v, _ := r.Float64()
			p.FocalLength = fmt.Sprintf("%.0fmm", math.Round(v))
		}
	}

	return p
}

func exifString(x *exif.Exif, name exif.FieldName) string {
	tag, err := x.Get(name)
	if err != nil {
		return ""
	}
	v, err := tag.StringVal()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.Trim(v, "\x00"))
}

// SaveImageFile 把磁盘上的图片复制进相册目录（可转换格式自动转 WebP），返回站点相对路径与 EXIF 元信息。
func (s *GalleryService) SaveImageFile(ctx context.Context, sourcePath string) (string, domain.GalleryPhoto, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", domain.GalleryPhoto{}, err
	}
	if !info.Mode().IsRegular() {
		return "", domain.GalleryPhoto{}, fmt.Errorf("%s is not a regular file", sourcePath)
	}
	meta := s.ExtractMetadata(sourcePath)
	image, err := s.saveImageFromFile(sourcePath)
	return image, meta, err
}

// SaveImageBytes 保存前端粘贴/拖拽的图片字节（base64 解码后传入）。
func (s *GalleryService) SaveImageBytes(ctx context.Context, name string, data []byte) (string, domain.GalleryPhoto, error) {
	if len(data) == 0 {
		return "", domain.GalleryPhoto{}, fmt.Errorf("empty image data")
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
		return "", domain.GalleryPhoto{}, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", domain.GalleryPhoto{}, err
	}
	if err := tmp.Close(); err != nil {
		return "", domain.GalleryPhoto{}, err
	}

	meta := s.ExtractMetadata(tmpPath)
	image, err := s.saveImageFromFile(tmpPath)
	return image, meta, err
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
