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

	//screenW, screenH := robotgo.GetScreenSize()
	screenW, screenH := 1280, 720
	maxScreenW, maxScreenH := robotgo.GetScreenSize()
	//TODO separate method
	// settings ////
	_, err := db.Exec("INSERT INTO video_track (clip_id, width, height) VALUES ($1, $2, $3)", clip.Id, screenW, screenH)
	utils.HandleError(err)
	////////////////

	//settingsFile, err := os.Create(filepath.Join(clip.TmpPath, "settings.csv")) //TODO couple domain and service around
	//utils.HandleError(err)
	//fmt.Fprintf(settingsFile, "%d,%d\n", screenW, screenH)
	//settingsFile.Close()

	go processFramesQueue(clip, framesQueue, doneChannel)

	seq := 0
	sleepInterval, fps := getSleepInterval()
	var prevTs int64 = -1

	for !isClipStopped(clip) {
		ts := utils.Mills()
		mouseX, mouseY := robotgo.GetMousePos()

		screenX := minInt(maxInt(mouseX-screenW/2, 0), maxScreenW-screenW)
		screenY := minInt(maxInt(mouseY-screenH/2, 0), maxScreenH-screenH)
		cbitm := robotgo.CaptureScreen(screenX, screenY, screenW, screenH)

		bitmBytes := robotgo.ToBitmapBytes(cbitm)
		robotgo.FreeBitmap(cbitm)

		framesQueue.Enqueue(&screenFrame{
			bitmBytes,
			mouseX - screenX,
			mouseY - screenY,
			ts,
			seq,
			fps,
		})

		sleepInterval, fps = getSleepInterval()

		if prevTs < 0 {
			prevTs = ts
		}

		targetSleepInterval := time.Duration(sleepInterval.Milliseconds()-(utils.Mills()-ts)) * time.Millisecond
		//fmt.Printf("> interval=%d, diff=%d\n", targetSleepInterval.Milliseconds(), utils.Mills()-ts)

		seq++

		if targetSleepInterval > 0 {
			fmt.Printf("> interval=%d\n", targetSleepInterval.Milliseconds())
			time.Sleep(targetSleepInterval)
		}
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
		return utils.Fps(12), 12
	case domain.RECORDER_SPEED_RARE:
		return utils.Fps(2), 2
	default:
		return utils.Fps(8), 8
	}
}

func maxInt(i1, i2 int) int {
	if i1 > i2 {
		return i1
	} else {
		return i2
	}
}

func minInt(i1, i2 int) int {
	if i1 < i2 {
		return i1
	} else {
		return i2
	}
}
