package exporter

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nantipov/longscreen/internal/domain"
	"github.com/nantipov/longscreen/internal/service"
	"github.com/nantipov/longscreen/internal/utils"
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

		exportScreenClip(db, clipId, fmt.Sprintf("clip%d", clipId)) //TODO do not compute it here
	}
}

func exportScreenClip(db *sql.DB, clipId int64, clipPath string) {
	rows, err := db.Query("SELECT f.filename FROM frame f WHERE f.clip_id = ? ORDER BY f.ts ASC", clipId)
	utils.HandleError(err)
	defer rows.Close()

	concatFilename := filepath.Join(clipPath, domain.DIR_TMP, "concat.list")
	concatFile, err := os.Create(concatFilename)
	utils.HandleError(err)
	for rows.Next() {
		var filename string
		err = rows.Scan(&filename)
		utils.HandleError(err)

		_, err = concatFile.WriteString(fmt.Sprintf("file '%s'\n", filepath.Join("..", "..", clipPath, domain.DIR_IMAGES, filename)))
		utils.HandleError(err)
	}
	utils.HandleError(concatFile.Close())

	outputVideoFilename := filepath.Join(clipPath, domain.DIR_TMP, "video_output.mp4")
	ffmpegCmd := exec.Command("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", concatFilename, "-vf", "fps=24", outputVideoFilename)

	var out, errs bytes.Buffer
	ffmpegCmd.Stdout = &out
	ffmpegCmd.Stderr = &errs
	err = ffmpegCmd.Run()
	fmt.Printf("out = %s, err = %s", out.String(), errs.String())
	utils.HandleError(err)
}
