package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	_ "golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

const (
	windowWidth  = 600
	windowHeight = 400
)

type Game struct{}

func (g *Game) Update() error {
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	bgColor := color.RGBA{0, 0, 0, 255}
	screen.Fill(bgColor)
	fontFace := basicfont.Face7x13

	white := color.RGBA{255, 255, 255, 255}
	green := color.RGBA{0, 255, 0, 255}

	text.Draw(screen, "Ninja Gaiden (NES)", fontFace, 220, 20, white)
	text.Draw(screen, "Any%", fontFace, 270, 40, white)
	text.Draw(screen, "22/286", fontFace, 270, 60, white)

	text.Draw(screen, "Act 1 ~ The Barbarian        49.00       49.00", fontFace, 50, 100, white)
	text.Draw(screen, "Act 2 ~ Bomberhead         1:57.00     2:46.00", fontFace, 50, 120, white)
	text.Draw(screen, "Act 3 ~ Basaquer           1:33.00     4:19.00", fontFace, 50, 140, white)
	text.Draw(screen, "Act 4 ~ Kelbeross          2:20.00     6:39.00", fontFace, 50, 160, white)
	text.Draw(screen, "Act 5 ~ Bloody Malth       3:04.00     9:43.00", fontFace, 50, 180, white)
	text.Draw(screen, "Act 6 ~ The Masked Devi-   2:48.00    12:31.00", fontFace, 50, 200, white)
	text.Draw(screen, "Act 6 ~ Jaquio               28.00    12:59.00", fontFace, 50, 220, white)
	text.Draw(screen, "Act 6 ~ The Demon           31.00    13:30.00", fontFace, 50, 240, white)

	text.Draw(screen, "0.00", fontFace, 270, 300, green)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return windowWidth, windowHeight
}

func main() {
	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle("Ninja Gaiden Split Timer")
	game := &Game{}

	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
