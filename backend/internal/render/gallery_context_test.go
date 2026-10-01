package render

import (
	"reflect"
	"testing"

	"gridea-pro/backend/internal/template"
)

// 相册模块的模板变量 photos 必须在上下文中暴露，否则主题 {% if photos %} 恒为假。
func TestBuildContextExposesPhotos(t *testing.T) {
	r := &Jinja2Renderer{}
	data := &template.TemplateData{
		Photos: []template.GalleryView{
			{ID: "a", Image: "/images/gallery/x.webp", Title: "t", Location: "合肥", IsTop: true},
		},
	}

	ctx := r.buildContext(data)

	v, ok := ctx["photos"]
	if !ok {
		t.Fatal("context missing key 'photos'")
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice || rv.Len() != 1 {
		t.Fatalf("photos not exposed as a 1-element slice: %#v", v)
	}
}
