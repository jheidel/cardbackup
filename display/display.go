package lcd

import (
	"fmt"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"strings"
	"time"

	"github.com/goiot/devices/monochromeoled"
	"golang.org/x/exp/io/i2c"

	log "github.com/sirupsen/logrus"
)

type Display struct {
	oled *monochromeoled.OLED

	done chan chan bool
}

func NewDisplay() (*Display, error) {
	oled, err := monochromeoled.Open(&i2c.Devfs{Dev: "/dev/i2c-1"})
	if err != nil {
		return nil, fmt.Errorf("opening oled: %v", err)
	}
	if err := oled.On(); err != nil {
		return nil, fmt.Errorf("turning on oled: %v", err)
	}
	if err := oled.Clear(); err != nil {
		return nil, fmt.Errorf("clearing oled: %v", err)
	}

	d := &Display{
		oled: oled,
		done: make(chan chan bool),
	}
	go d.loop()
	return d, nil
}

func (d *Display) cleanup() error {
	defer d.oled.Close()
	if err := d.oled.Clear(); err != nil {
		return err
	}
	if err := d.oled.Off(); err != nil {
		return err
	}
	return nil
}

func (d *Display) Close() {
	log.Infof("Shutting down display")
	c := make(chan bool)
	d.done <- c
	<-c
	log.Infof("Display shut down")
}

func addLabel(img *image.RGBA, x, y int, label string) {
	col := color.RGBA{255, 255, 255, 255}
	point := fixed.Point26_6{fixed.Int26_6(x * 64), fixed.Int26_6(y * 64)}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(label)
}

func makeImage(line1, line2 string) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 128, 64))
	addLabel(img, 0, 20, "Hello World!")
	addLabel(img, 0, 20+13+2, line1)
	addLabel(img, 0, 20+13+2+13+2, line2)
	return img
}

func (d *Display) draw() error {
	tss := strings.Split(time.Now().String(), " ")

	img := makeImage(tss[0], tss[1])
	if err := d.oled.SetImage(0, 0, img); err != nil {
		return err
	}

	if err := d.oled.Draw(); err != nil {
		return err
	}

	return nil
}

func (d *Display) loop() {
	tick := time.NewTicker(time.Millisecond * 50)
	for {
		select {
		case c := <-d.done:
			if err := d.cleanup(); err != nil {
				log.Warnf("Error closing display: %v", err)
			}
			c <- true
			return
		case <-tick.C:
			if err := d.draw(); err != nil {
				log.Warnf("Error drawing display: %v", err)
			}
		}
	}
}
