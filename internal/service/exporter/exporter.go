package exporter

import (
	"bufio"
	"bytes"
	"database/sql"
	"fmt"
	"image/color"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fogleman/gg"
	"golang.org/x/image/bmp"

	"github.com/nantipov/longscreen/internal/domain"
	"github.com/nantipov/longscreen/internal/service"
	"github.com/nantipov/longscreen/internal/utils"
)

type resultResolution struct {
	w int
	h int
}

var (
	allowedResolutions []*resultResolution = []*resultResolution{ //TODO: examine if more grained resolutions exist or rule/law?
		&resultResolution{w: 176, h: 144},
		&resultResolution{w: 320, h: 240},
		&resultResolution{w: 352, h: 288},
		&resultResolution{w: 640, h: 480},
		&resultResolution{w: 1280, h: 720},
		&resultResolution{w: 1920, h: 1080},
		&resultResolution{w: 3840, h: 2160},
	}
	// https://stackoverflow.com/questions/54602439/ffmpeg-is-creating-invalid-output-when-using-exotic-resolutions

	mouseColorLine   = color.RGBA{255, 255, 0, 255}
	mouseColorCircle = color.RGBA{255, 255, 0, 127}
)

func Export() {
	db := service.GetDatabase() //TODO status - constant
	rows, err := db.Query(`
	    SELECT t.id, t.clip_id, t.width, t.height
		FROM clip c, video_track t
		WHERE
		  c.status = 'STOPPED'
		  AND c.clip_type = $1
		  AND t.clip_id = c.id
	`, domain.CLIP_TYPE_SCREEN)
	utils.HandleError(err)
	videoTracks := make([]*domain.VideoTrack, 0)
	for rows.Next() {
		var id, clipId int64
		var width, height int
		err = rows.Scan(&id, &clipId, &width, &height)
		utils.HandleError(err)
		videoTracks = append(videoTracks, &domain.VideoTrack{id, clipId, width, height})
	}
	rows.Close()
	for _, vt := range videoTracks {
		exportScreenClip(vt, db)
	}
}

func exportScreenClip(videoTrack *domain.VideoTrack, db *sql.DB) {
	fmt.Printf(">>> exportScreenClip %d | %d\n", videoTrack.ClipId, videoTrack.Id)
	clipPath := fmt.Sprintf("clip%d", videoTrack.ClipId) //TODO do not compute it here

	importFrames(db, videoTrack.Id, clipPath)

	rows, err := db.Query(`
	  SELECT f.id, f.x_mouse, f.y_mouse, f.seq, f.fps, f.ts
	  FROM frame f
	  WHERE f.video_track_id = $1 AND f.status = 'CREATED'
	  ORDER BY seq ASC
	`, videoTrack.Id)
	utils.HandleError(err)

	frames := make([]*domain.VideoFrame, 0)
	var seq, mouseX, mouseY, fps int
	var frameId, ts int64
	for rows.Next() {
		err = rows.Scan(&frameId, &mouseX, &mouseY, &seq, &fps, &ts)
		utils.HandleError(err)

		frames = append(frames, &domain.VideoFrame{frameId, seq, mouseX, mouseY, fps, ts})
	}
	rows.Close()

	frameUpdtStmt, err := db.Prepare("UPDATE frame SET status = 'PROCESSED' WHERE id = ?")
	utils.HandleError(err)
	defer frameUpdtStmt.Close()

	overridenResolution := getOverridenResolution(videoTrack.Width, videoTrack.Height)
	dc := gg.NewContext(overridenResolution.w, overridenResolution.h)
	mouseDeltaX := overridenResolution.w/2 - videoTrack.Width/2
	mouseDeltaY := overridenResolution.h/2 - videoTrack.Height/2
	bufStartFrame := 0
	bufEndFrame := 0
	prevFps := -1
	var prevTs int64 = -1
	section := 0
	sectionSeq := 0

	videoFiles := make([]string, 0)

	for _, frame := range frames {
		if prevFps < 0 {
			prevFps = frame.Fps
		}

		if prevTs < 0 {
			prevTs = frame.Ts
		}

		fmt.Printf(">>> interval real=%d expected=%d\n", frame.Ts-prevTs, utils.Fps(prevFps).Milliseconds())

		if sectionSeq > 0 && frame.Ts-prevTs > utils.Fps(prevFps).Milliseconds()*2 {
			originalSectionSeq := sectionSeq - 1
			originalSectionFilename := getFrameInSectionFilename(clipPath, section, originalSectionSeq)
			extraFramesToAdd := int((frame.Ts-prevTs)/utils.Fps(prevFps).Milliseconds()) - 1
			for copyIndex := 0; copyIndex < extraFramesToAdd; copyIndex++ {
				println("Adding extra frame")
				copyFile(originalSectionFilename, getFrameInSectionFilename(clipPath, section, sectionSeq))
				sectionSeq = sectionSeq + 1
			}
		}

		if frame.Fps != prevFps && bufEndFrame > bufStartFrame {
			videoFiles = append(videoFiles, exportToVideoFile(clipPath, section, bufStartFrame, bufEndFrame, prevFps))
			bufStartFrame = frame.Seq
			bufEndFrame = 0
			section = section + 1
			sectionSeq = 0
		}

		prevFps = frame.Fps
		prevTs = frame.Ts

		originalImageFilename := filepath.Join(clipPath, domain.DIR_IMAGES, fmt.Sprintf("image-%d.bmp", frame.Seq))

		img, err := gg.LoadImage(originalImageFilename)
		if err == nil {
			dc.DrawImage(img, mouseDeltaX, mouseDeltaY)

			dc.SetColor(mouseColorLine)
			dc.SetLineWidth(3.0)
			dc.DrawCircle(float64(frame.MouseX+mouseDeltaX), float64(frame.MouseY+mouseDeltaY), 20.0)

			dc.SetColor(mouseColorCircle)
			dc.Fill()

			//dc.SavePNG(filepath.Join(clipPath, domain.DIR_TMP, fmt.Sprintf("image-%d.png", seq)))
			//TODO store to bmp as long original file is removed anyway?
			//TODO reduce load because of compression
			outputFile, err := os.Create(getFrameInSectionFilename(clipPath, section, sectionSeq))
			utils.HandleError(err)
			err = bmp.Encode(outputFile, dc.Image())
			utils.HandleError(err)
			outputFile.Close()

			os.Remove(originalImageFilename)
		}
		bufEndFrame = frame.Seq
		sectionSeq = sectionSeq + 1

		_, err = frameUpdtStmt.Exec(frame.Id)
		utils.HandleError(err)
	}

	if bufEndFrame > bufStartFrame {
		videoFiles = append(videoFiles, exportToVideoFile(clipPath, section, bufStartFrame, bufEndFrame, prevFps))
	}

	if len(videoFiles) > 0 {
		buildOutputVideoFile(videoFiles, videoTrack.ClipId, db)
	}
}

func getFrameInSectionFilename(clipPath string, section int, sectionSeq int) string {
	return filepath.Join(clipPath, domain.DIR_TMP, fmt.Sprintf("image-%d-%d.bmp", section, sectionSeq))
}

func exportToVideoFile(clipPath string, section int, startFrame int, stopFrame int, fps int) string {
	filename := fmt.Sprintf("video_output_%d_%d_%d_%d.ts", section, startFrame, stopFrame, fps)
	outputVideoFilename := filepath.Join(clipPath, domain.DIR_TMP, filename)
	println(">>>   " + outputVideoFilename)
	inputImagesFilemask := filepath.Join(clipPath, domain.DIR_TMP, fmt.Sprintf("image-%d-%%d.bmp", section))
	ffmpegCmd := exec.Command(
		"ffmpeg", "-y", "-framerate", fmt.Sprintf("%d", fps),
		"-i", inputImagesFilemask,
		/*"-start_number", fmt.Sprintf("%d", startFrame),*/ /* "-frames:v", fmt.Sprintf("%d", stopFrame-startFrame),*/
		/*"-c:v", "libx264",*/
		"-vf", "fps=24,format=yuv420p", //libx264
		outputVideoFilename)

	for _, a := range ffmpegCmd.Args {
		fmt.Printf("%s ", a)
	}
	fmt.Println()

	var out, errs bytes.Buffer
	ffmpegCmd.Stdout = &out
	ffmpegCmd.Stderr = &errs
	err := ffmpegCmd.Run()
	//TODO remove tmp files
	if err != nil {
		fmt.Printf("out = %s, err = %s", out.String(), errs.String())
	}
	return outputVideoFilename
}

func buildOutputVideoFile(videoFiles []string, clipId int64, db *sql.DB) {
	outputVideoFilename := fmt.Sprintf("video_output_%d.mp4", clipId)

	println(">>>   " + outputVideoFilename)
	ffmpegCmd := exec.Command(
		"ffmpeg", "-y",
		"-i", "concat:"+strings.Join(videoFiles, "|"),
		"-c:v", "libx264", outputVideoFilename)
	var out, errs bytes.Buffer
	ffmpegCmd.Stdout = &out
	ffmpegCmd.Stderr = &errs

	for _, a := range ffmpegCmd.Args {
		fmt.Printf("%s ", a)
	}
	fmt.Println()

	err := ffmpegCmd.Run()
	//TODO remove tmp files
	if err != nil {
		fmt.Printf("out = %s, err = %s", out.String(), errs.String())
	}
}

func importFrames(db *sql.DB, videoTrackId int64, clipPath string) {
	fmt.Printf(">>> importFrames %s | %d\n", clipPath, videoTrackId)
	framesFilename := filepath.Join(clipPath, domain.DIR_TMP, "frames.csv") //TODO service with domain, filename constant?
	framesFileInfo, err := os.Stat(framesFilename)
	if os.IsNotExist(err) || framesFileInfo.IsDir() {
		return
	}

	framesFile, err := os.Open(framesFilename)
	framesScanner := bufio.NewScanner(framesFile)

	tx, err := db.Begin()
	utils.HandleError(err)

	stmt, err := tx.Prepare("INSERT OR IGNORE INTO frame (video_track_id, x_mouse, y_mouse, fps, ts, seq) VALUES (?, ?, ?, ?, ?, ?)")
	utils.HandleError(err)

	var seq, mouseX, mouseY, fps int
	var ts int64

	for framesScanner.Scan() {
		line := framesScanner.Text()
		println(line)
		_, err = fmt.Sscanf(line, "%d,%d,%d,%d,%d", &seq, &ts, &mouseX, &mouseY, &fps)
		utils.HandleError(err)
		res, err := stmt.Exec(videoTrackId, mouseX, mouseY, fps, ts, seq)
		utils.HandleError(err)

		id0, _ := res.LastInsertId()
		rows0, _ := res.RowsAffected()
		fmt.Printf(">>> insert result %d, %d\n", id0, rows0)
	}

	stmt.Close()
	utils.HandleError(tx.Commit())
	framesFile.Close()
	os.Remove(framesFilename)
}

func getOverridenResolution(captureW, captureH int) *resultResolution { //TODO rename adapt resolution?
	for _, r := range allowedResolutions {
		if r.w >= captureW && r.h >= captureH {
			return r
		}
	}
	return &resultResolution{
		captureW,
		captureH,
	}
}

func copyFile(src, dst string) error {
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
	if err != nil {
		return err
	}
	return out.Close()
}
