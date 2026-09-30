package datepicker

import (
	"embed"
	"image/png"
	"path"

	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed resource/*.png
var imageResource embed.FS

type imageCacheKey struct {
	name      string
	colorMode ebiten.ColorMode
}

type resourceImages struct {
	m map[imageCacheKey]*ebiten.Image
}

var theResourceImages = &resourceImages{}

func (i *resourceImages) Get(name string, colorMode ebiten.ColorMode) (*ebiten.Image, error) {
	key := imageCacheKey{
		name:      name,
		colorMode: colorMode,
	}
	if img, ok := i.m[key]; ok {
		return img, nil
	}

	f, err := imageResource.Open(path.Join("resource", name+".png"))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	pImg, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	pImg = basicwidget.CreateMonochromeImage(colorMode, pImg)
	img := ebiten.NewImageFromImage(pImg)
	if i.m == nil {
		i.m = map[imageCacheKey]*ebiten.Image{}
	}
	i.m[key] = img
	return img, nil
}
