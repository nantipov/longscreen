package recorder

import (
	"fmt"
	"image/color"

	"github.com/nantipov/longscreen/internal/utils"

	"os"
	"path/filepath"
	"time"

	"github.com/nantipov/longscreen/internal/domain"

	"github.com/fogleman/gg"
	"github.com/nantipov/longscreen/internal/service"

	"github.com/enriquebris/goconcurrentqueue"
	"github.com/go-vgo/robotgo"

	"golang.org/x/image/bmp"
)

type resultResolution struct {
	w int
	h int
}

type screenFrame struct {
	bitmBytes []byte
	mouseX    int
	mouseY    int
	ts        int64
	seq       int
}

var allowedResolutions []*resultResolution = []*resultResolution{
	&resultResolution{w: 640, h: 480},
	&resultResolution{w: 1280, h: 720},
	&resultResolution{w: 1920, h: 1080},
	&resultResolution{w: 3840, h: 2160},
}

var mouseColorLine = color.RGBA{255, 255, 0, 255}
var mouseColorCircle = color.RGBA{255, 255, 0, 127}

func RecordScreen(clip *domain.Clip) {
	db := service.GetDatabase()

	defer markClipAsStopped(clip.Id, db)

	doneChannel := make(chan bool, 1)
	framesQueue := goconcurrentqueue.NewFIFO()

	screenW, screenH := robotgo.GetScreenSize()
	overridenResolution := getOverridenResolution(screenW, screenH)

	go processFramesQueue(clip, framesQueue, overridenResolution, doneChannel)

	seq := 0
	sleepInterval := getSleepInteval()

	for !isClipStopped(clip) {
		mouseX, mouseY := robotgo.GetMousePos()
		cbitm := robotgo.CaptureScreen()
		bitmBytes := robotgo.ToBitmapBytes(cbitm)
		robotgo.FreeBitmap(cbitm)
		//robotgo.SaveCapture(fmt.Sprintf("s%d", seq))

		framesQueue.Enqueue(&screenFrame{
			bitmBytes,
			mouseX,
			mouseY,
			time.Now().Unix(), //TODO unix time?
			seq,
		})

		sleepInterval = getSleepInteval()

		seq++
		time.Sleep(sleepInterval)
	}
	fmt.Printf("Seq=%d\n", seq)

	<-doneChannel
}

func processFramesQueue(
	clip *domain.Clip, queue *goconcurrentqueue.FIFO,
	overridenResolution *resultResolution, doneChannel chan bool) {

	isIdle := false

	logFile, err := os.Create(filepath.Join(clip.TmpPath, "frames.csv"))
	utils.HandleError(err)
	defer logFile.Close()

	for {
		frame, err := queue.Dequeue()
		if err != nil {
			if isIdle {
				break
			} else {
				isIdle = true
				time.Sleep(5 * time.Second) //TODO constant or so
			}
		} else {
			isIdle = false
			frameValue := frame.(*screenFrame)
			logFile.WriteString(fmt.Sprintf("%d,%d,%d,%d\n", frameValue.seq, frameValue.ts, frameValue.mouseX, frameValue.mouseY))
			saveFrame(frameValue, clip, overridenResolution)
		}
		fmt.Printf("Queue size=%d\n", queue.GetLen())
	}

	doneChannel <- true
}

func saveFrame(frame *screenFrame, clip *domain.Clip, overridenResolution *resultResolution) {
	robotgo.SaveImg(frame.bitmBytes, filepath.Join(clip.ImagesPath, fmt.Sprintf("image-%d.bmp", frame.seq)))
}

func saveFrame0(frame *screenFrame, clip *domain.Clip, overridenResolution *resultResolution) {
	var dc *gg.Context
	var mouseDeltaX, mouseDeltaY int
	if overridenResolution == nil {
		// dc = gg.NewContext(frame.image.Bounds().Dx(), frame.image.Bounds().Dy())
		mouseDeltaX = 0
		mouseDeltaY = 0
	} else {
		// dc = gg.NewContext(overridenResolution.w, overridenResolution.h)
		//mouseDeltaX = overridenResolution.w/2 - frame.image.Bounds().Dx()/2
		//mouseDeltaY = overridenResolution.h/2 - frame.image.Bounds().Dy()/2
	}

	// dc.DrawImage(frame.image, mouseDeltaX, mouseDeltaY)

	dc.SetColor(mouseColorLine)
	dc.SetLineWidth(3.0)
	dc.DrawCircle(float64(frame.mouseX+mouseDeltaX), float64(frame.mouseY+mouseDeltaY), 20.0)

	dc.SetColor(mouseColorCircle)
	dc.Fill()

	//dc.SavePNG(filepath.Join(clip.ImagesPath, fmt.Sprintf("image-%d.png", frame.seq)))

	file, _ := os.Create(filepath.Join(clip.ImagesPath, fmt.Sprintf("image-%d.bmp", frame.seq)))
	bmp.Encode(file, dc.Image())
	file.Close()
}

func getSleepInteval() time.Duration {
	speed := service.GetGlobalSettings().GetSpeed()
	switch speed {
	case domain.RECORDER_SPEED_REALTIME:
		// 24 fps
		return 1000 / 24 * time.Millisecond
	case domain.RECORDER_SPEED_RARE:
		// 2 fps
		return 1000 / 2 * time.Millisecond
	default:
		// 12 fps
		return 1000 / 12 * time.Millisecond
	}
}

func getOverridenResolution(captureW, captureH int) *resultResolution {
	for _, r := range allowedResolutions {
		if r.w == captureW && r.h == captureH {
			return nil
		}
		if r.w >= captureW && r.h >= captureH {
			return r
		}
	}
	return nil
}
