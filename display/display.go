package lcd

import (
	"fmt"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"sync"
	"time"

	"cardbackup/backup"
	"cardbackup/filesystem"
	"cardbackup/util"

	"github.com/goiot/devices/monochromeoled"
	"golang.org/x/exp/io/i2c"

	log "github.com/sirupsen/logrus"
)

type Display struct {
	oled *monochromeoled.OLED

	lock     sync.Mutex
	fs       *filesystem.Filesystems
	p        *backup.BackupProgress
	txfrDone bool
	drawErr  error
	drawErrT time.Time

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

func (d *Display) SetFilesystems(fs *filesystem.Filesystems) {
	if fs == nil {
		return
	}
	d.lock.Lock()
	defer d.lock.Unlock()
	d.fs = fs
}

func (d *Display) SetProgress(p *backup.BackupProgress) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.p = p
}

func (d *Display) SetProgressDone() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.p.Percent = 100
	d.txfrDone = true
}

func (d *Display) ResetState() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.txfrDone = false
	d.p = nil
	d.drawErr = nil
}

func (d *Display) SetError(err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.drawErr = err
	d.drawErrT = time.Now()
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
	c := make(chan bool)
	d.done <- c
	<-c
}

func addLabel(img *image.RGBA, label string, x, y int) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{255, 255, 255, 255}),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(label)
}

func addCenteredLabel(img *image.RGBA, label string, y int) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{255, 255, 255, 255}),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(0, y),
	}
	advance := d.MeasureString(label)
	d.Dot.X = fixed.I(128)/2 - advance/2
	d.DrawString(label)
}

func addProgressLabel(img *image.RGBA, percent int32, y int) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{255, 255, 255, 255}),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(0, y),
	}
	label := fmt.Sprintf("%d%%", percent)
	advance := d.MeasureString(label)
	d.Dot.X = fixed.I(128) - advance
	barWidth := d.Dot.X.Floor()
	d.DrawString(label)

	draw.Draw(img,
		image.Rectangle{
			Min: image.Point{
				X: 0,
				Y: y - 10,
			},
			Max: image.Point{
				X: barWidth - 2,
				Y: y,
			},
		},
		image.NewUniform(color.RGBA{255, 255, 255, 255}), image.ZP, draw.Src)

	draw.Draw(img,
		image.Rectangle{
			Min: image.Point{
				X: 1,
				Y: y - 9,
			},
			Max: image.Point{
				X: barWidth - 3,
				Y: y - 1,
			},
		},
		image.NewUniform(color.RGBA{0, 0, 0, 0}), image.ZP, draw.Src)

	progressWidth := (barWidth - 4) * int(percent) / 100

	draw.Draw(img,
		image.Rectangle{
			Min: image.Point{
				X: 2,
				Y: y - 8,
			},
			Max: image.Point{
				X: progressWidth,
				Y: y - 2,
			},
		},
		image.NewUniform(color.RGBA{255, 255, 255, 255}), image.ZP, draw.Src)
}

// page1 displays filesystem detection status and instructions.
func (d *Display) page1(line3 string) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 128, 64))

	d.lock.Lock()
	defer d.lock.Unlock()

	l1 := "Drive: --"
	if d.fs != nil && d.fs.Dst != nil {
		l1 = fmt.Sprintf("Drive: %s", util.BytesToString(d.fs.Dst.Used))
	}

	l2 := "Card:  --"
	if d.fs != nil && d.fs.Src != nil {
		l2 = fmt.Sprintf("Card:  %s", util.BytesToString(d.fs.Src.Used))
	}

	addLabel(img, l1, 0, 14)
	addLabel(img, l2, 0, 14*2)

	addCenteredLabel(img, line3, 14*3+7)
	return img
}

// dots creates an animated `...` string.
func dots() string {
	n := time.Now().UnixNano() * 2 / 1e9
	switch {
	case n%4 == 1:
		return ".  "
	case n%4 == 2:
		return ".. "
	case n%4 == 3:
		return "..."
	default:
		return "   "
	}
}

// page2 displays transfer status.
func (d *Display) page2() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 128, 64))

	d.lock.Lock()
	defer d.lock.Unlock()

	if d.txfrDone {
		addCenteredLabel(img, "Done!", 14)
	} else {
		addCenteredLabel(img, "Transferring"+dots(), 14)
	}
	addProgressLabel(img, d.p.Percent, 14*2)

	fmtDur := func(d time.Duration) string {
		return util.TruncateSeconds(d).String()
	}

	l3 := fmt.Sprintf("ETA: %s", fmtDur(d.p.Remaining))
	if d.txfrDone {
		l3 = fmt.Sprintf("%s in %s", util.BytesToString(d.p.BytesSent), fmtDur(d.p.Elapsed))
		addCenteredLabel(img, l3, 14*3)
	} else {
		addLabel(img, l3, 0, 14*3)
	}

	if d.fs != nil && d.fs.Dst != nil {
		l4 := fmt.Sprintf("Drive: %s", util.BytesToString(d.fs.Dst.Used))
		addLabel(img, l4, 0, 14*4)
	}
	return img
}

// pageError shows an error message after a failed transfer.
func (d *Display) pageError() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 128, 64))

	d.lock.Lock()
	defer d.lock.Unlock()

	msg := fmt.Sprintf("ERROR! %s", d.drawErr)

	t := int64(time.Now().Sub(d.drawErrT).Seconds())
	if t%7 == 4 || t%7 == 5 || t%7 == 6 {
		msg = "Disconnect and reconnect the card to try again."
	}

	words := strings.Fields(msg)
	dr := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{255, 255, 255, 255}),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(0, 14),
	}

	row := 1
	for _, w := range words {
		if row > 4 {
			break
		}
		advance := dr.MeasureString(w)
		if dr.Dot.X+advance > fixed.I(128) {
			row += 1
			dr.Dot = fixed.P(0, row*14)
		}
		dr.DrawString(w + " ")
	}
	return img
}

func (d *Display) makeImage() *image.RGBA {
	switch {
	case d.drawErr != nil:
		return d.pageError()
	case d.fs == nil:
		return d.page1("Please wait...")
	case d.fs.Dst == nil:
		return d.page1("* Connect Drive *")
	case d.fs.Src == nil:
		return d.page1("* Connect Card *")
	case d.p == nil:
		return d.page1("Starting...")
	default:
		return d.page2()
	}
}

func (d *Display) draw() error {
	img := d.makeImage()
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
