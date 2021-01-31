package recorder

import (
	"fmt"

	"github.com/nantipov/longscreen/internal/utils"

	"os"
	"path/filepath"
	"time"

	"github.com/nantipov/longscreen/internal/domain"

	"github.com/nantipov/longscreen/internal/service"

	"github.com/enriquebris/goconcurrentqueue"
	"github.com/go-vgo/robotgo"
)

type screenFrame struct {
	bitmBytes []byte
	mouseX    int
	mouseY    int
	ts        int64
	seq       int
	fps       int
}

func RecordScreen(clip *domain.Clip) {
	db := service.GetDatabase()

	defer markClipAsStopped(clip.Id, db)

	doneChannel := make(chan bool, 1) //TODO close channel
	framesQueue := goconcurrentqueue.NewFIFO()

	screenW, screenH := robotgo.GetScreenSize()

	settingsFile, err := os.Create(filepath.Join(clip.TmpPath, "settings.csv")) //TODO couple domain and service around
	utils.HandleError(err)
	fmt.Fprintf(settingsFile, "%d,%d\n", screenW, screenH)
	settingsFile.Close()

	go processFramesQueue(clip, framesQueue, doneChannel)

	seq := 0
	sleepInterval, fps := getSleepInterval()

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
			time.Now().Unix(), //TODO unix time? do we need it for any calculations?
			seq,
			fps,
		})

		sleepInterval, fps = getSleepInterval() //TODO expose interval/fps in the frames log?

		seq++
		time.Sleep(sleepInterval)
	}
	fmt.Printf("Seq=%d\n", seq)

	<-doneChannel
}

func processFramesQueue(
	clip *domain.Clip, queue *goconcurrentqueue.FIFO, doneChannel chan bool) {

	isIdle := false

	logFile, err := os.Create(filepath.Join(clip.TmpPath, "frames.csv")) // tmp/ ? logFile or framesFile?
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
			logFile.WriteString(fmt.Sprintf("%d,%d,%d,%d,%d\n", frameValue.seq, frameValue.ts, frameValue.mouseX, frameValue.mouseY, frameValue.fps))
			saveFrame(frameValue, clip)
		}
		fmt.Printf("Queue size=%d\n", queue.GetLen())
	}

	doneChannel <- true
}

func saveFrame(frame *screenFrame, clip *domain.Clip) {
	robotgo.SaveImg(frame.bitmBytes, filepath.Join(clip.ImagesPath, fmt.Sprintf("image-%d.bmp", frame.seq)))
}

func getSleepInterval() (time.Duration, int) {
	speed := service.GetGlobalSettings().GetSpeed()
	switch speed {
	case domain.RECORDER_SPEED_OFTEN:
		return fps(12)
	case domain.RECORDER_SPEED_RARE:
		return fps(2)
	default:
		return fps(8)
	}
}

func fps(fps int) (time.Duration, int) {
	return time.Duration(1000/fps) * time.Millisecond, fps
}
