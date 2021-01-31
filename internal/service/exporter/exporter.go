package exporter

import (
	"bufio"
	"bytes"
	_ "database/sql" //TODO do we need it?
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"

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
	db := service.GetDatabase()
	rows, err := db.Query("SELECT c.id, c.path FROM clip c WHERE c.is_stopped > 0 AND c.is_exported = 0 AND c.clip_type = ?", domain.CLIP_TYPE_SCREEN)
	utils.HandleError(err)

	defer rows.Close()

	for rows.Next() {
		var clipId int64
		var clipPath string
		err = rows.Scan(&clipId, &clipPath) //TODO clipPath has incorrect value
		utils.HandleError(err)

		exportScreenClip(fmt.Sprintf("clip%d", clipId)) //TODO do not compute it here
	}
}

func exportScreenClip(clipPath string) {
	settingsFile, err := os.Open(filepath.Join(clipPath, domain.DIR_TMP, "settings.csv")) //TODO couple domain and service around
	defer settingsFile.Close()
	settingsScanner := bufio.NewScanner(settingsFile)
	var screenW, screenH int
	if settingsScanner.Scan() {
		line := settingsScanner.Text()
		_, err = fmt.Sscanf(line, "%d,%d", &screenW, &screenH)
		utils.HandleError(err)
	} else {
		println("Could not read settings")
		return
	}

	overridenResolution := getOverridenResolution(screenW, screenH)

	dc := gg.NewContext(overridenResolution.w, overridenResolution.h)
	mouseDeltaX := overridenResolution.w/2 - screenW/2
	mouseDeltaY := overridenResolution.h/2 - screenH/2

	framesFile, err := os.Open(filepath.Join(clipPath, domain.DIR_TMP, "frames.csv")) //TODO couple domain and service around
	framesScanner := bufio.NewScanner(framesFile)

	bufStartFrame := 0
	bufEndFrame := 0
	prevFps := -1

	var seq, mouseX, mouseY, fps int
	var ts int64

	for framesScanner.Scan() {
		line := framesScanner.Text()
		_, err = fmt.Sscanf(line, "%d,%d,%d,%d,%d", &seq, &ts, &mouseX, &mouseY, &fps)
		utils.HandleError(err)

		if prevFps < 0 {
			prevFps = fps
		}

		if fps != prevFps && bufEndFrame > bufStartFrame {
			exportToVideoFile(clipPath, bufStartFrame, bufEndFrame, prevFps)
			bufStartFrame = seq
			bufEndFrame = 0
		}

		prevFps = fps

		originalImageFilename := filepath.Join(clipPath, domain.DIR_IMAGES, fmt.Sprintf("image-%d.bmp", seq))

		img, err := gg.LoadImage(originalImageFilename)
		if err == nil {
			dc.DrawImage(img, mouseDeltaX, mouseDeltaY)

			dc.SetColor(mouseColorLine)
			dc.SetLineWidth(3.0)
			dc.DrawCircle(float64(mouseX+mouseDeltaX), float64(mouseY+mouseDeltaY), 20.0)

			dc.SetColor(mouseColorCircle)
			dc.Fill()

			//dc.SavePNG(filepath.Join(clipPath, domain.DIR_TMP, fmt.Sprintf("image-%d.png", seq)))
			//TODO store to bmp as long original file is removed anyway?
			//TODO reduce load because of compression
			outputFile, err := os.Create(filepath.Join(clipPath, domain.DIR_TMP, fmt.Sprintf("image-%d.bmp", seq)))
			utils.HandleError(err)
			err = bmp.Encode(outputFile, dc.Image())
			utils.HandleError(err)
			outputFile.Close()

			os.Remove(originalImageFilename)
		}

		bufEndFrame = seq
	}
	framesFile.Close()

	if bufEndFrame > bufStartFrame {
		exportToVideoFile(clipPath, bufStartFrame, bufEndFrame, prevFps)
	}
}

func exportToVideoFile(clipPath string, startFrame int, stopFrame int, fps int) {
	outputVideoFilename := filepath.Join(clipPath, domain.DIR_TMP, fmt.Sprintf("video_output_%d_%d_%d.mp4", startFrame, stopFrame, fps))
	println(">>>   " + outputVideoFilename)
	inputImagesFilemask := filepath.Join(clipPath, domain.DIR_TMP, "image-%d.bmp")
	ffmpegCmd := exec.Command(
		"ffmpeg", "-y", "-framerate", fmt.Sprintf("%d", fps),
		"-i", inputImagesFilemask,
		"-start_number", fmt.Sprintf("%d", startFrame), "-frames:v", fmt.Sprintf("%d", stopFrame-startFrame),
		"-c:v", "libx264", "-vf", "fps=24,format=yuv420p",
		outputVideoFilename)
	var out, errs bytes.Buffer
	ffmpegCmd.Stdout = &out
	ffmpegCmd.Stderr = &errs
	for _, a := range ffmpegCmd.Args {
		print(a + " ")
	}
	println()
	err := ffmpegCmd.Run()
	fmt.Printf("out = %s, err = %s", out.String(), errs.String())
	utils.HandleError(err)
	//TODO remove tmp files
}

func getOverridenResolution(captureW, captureH int) *resultResolution {
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
