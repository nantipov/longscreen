package recorder

import (
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/nantipov/longscreen/internal/domain"
	"github.com/nantipov/longscreen/internal/utils"
	"github.com/rostislaved/screenshot"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/mousebind"

	"github.com/fogleman/gg"
	"github.com/nantipov/longscreen/internal/service"
)

const FRAMES_BUF_SIZE = 5

type screenFrame struct {
	image  *image.RGBA
	mouseX int16
	mouseY int16
	ts     int64
}

func RecordScreen(clip *domain.Clip) {
	db := service.GetDatabase()

	defer markClipAsStopped(clip.Id, db)

	xconn := newXConn()
	defer xconn.conn.Close()

	screenshoter := screenshot.New()
	frames := make([]*screenFrame, FRAMES_BUF_SIZE)

	seq := 0
	frameBufIndex := 0
	sleepInterval := getSleepInteval()

	defer mousebind.UngrabPointer(xconn.utilConn)

	for !isClipStopped(clip) {
		img, err := screenshoter.CaptureScreen()
		utils.HandleError(err) //TODO handle error, stop recording?

		pointerCookie := xproto.QueryPointer(xconn.conn, xconn.screenInfo.Root)
		pointerReply, err := pointerCookie.Reply()
		utils.HandleError(err)

		//TODO mouse pointer query error handler
		mouseX := pointerReply.RootX
		mouseY := pointerReply.RootY

		frames[frameBufIndex] = &screenFrame{
			img,
			mouseX,
			mouseY,
			time.Now().Unix(), //TODO unix time?
		}

		frameBufIndex++
		if frameBufIndex == FRAMES_BUF_SIZE {
			go saveBufferedFrames(clip, seq, frames, db)
			frames = make([]*screenFrame, FRAMES_BUF_SIZE)
			frameBufIndex = 0
			sleepInterval = getSleepInteval()
		}
		seq++
		time.Sleep(sleepInterval)
	}
	//TODO: flush buffer after completion
}

func saveBufferedFrames(clip *domain.Clip, currentSeq int, frames []*screenFrame, db *sql.DB) {

	frameEntrySql := `
	INSERT INTO frame (clip_id, x_mouse, y_mouse, ts, filename)
	VALUES (?, ?, ?, ?, ?)
	`
	//tx, err := db.Begin()

	frameEntryStmt, err := db.Prepare(frameEntrySql)
	utils.HandleError(err)
	defer frameEntryStmt.Close()

	for n, frame := range frames {
		filename := fmt.Sprintf("image-%d.png", currentSeq-len(frames)+n+1)
		file, err := os.Create(filepath.Join(clip.ImagesPath, filename))
		if err != nil {
			panic(err)
		}
		defer file.Close()

		//TODO: draw cursor on compile/export-time
		//TODO: save frame data to database
		dc := gg.NewContextForRGBA(frame.image)
		dc.SetColor(color.RGBA{255, 255, 0, 127})

		dc.DrawCircle(float64(frame.mouseX), float64(frame.mouseY), 20.0)
		dc.SetColor(color.RGBA{255, 255, 0, 127})
		dc.Fill()
		png.Encode(file, frame.image)

		_, err = frameEntryStmt.Exec(clip.Id, frame.mouseX, frame.mouseY, frame.ts, filename)
		utils.HandleError(err)
	}

	//tx.Commit()
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
	case domain.RECORDER_SPEED_MIDDLE:
		// 12 fps
		return 1000 / 12 * time.Millisecond
	}
	// 12 fps
	return 1000 / 12 * time.Millisecond
}

type XConn struct {
	conn       *xgb.Conn
	screenInfo *xproto.ScreenInfo
	utilConn   *xgbutil.XUtil
}

func newXConn() *XConn {
	conn, err := xgb.NewConn()
	utils.HandleError(err)

	utilConn, err := xgbutil.NewConn()
	utils.HandleError(err)

	screenInfo := xproto.Setup(conn).DefaultScreen(conn)

	mousebind.Initialize(utilConn)

	return &XConn{
		conn:       conn,
		screenInfo: screenInfo,
		utilConn:   utilConn,
	}
}
