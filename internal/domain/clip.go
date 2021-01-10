package domain //TODO move to domain

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nantipov/longscreen/internal/utils"
)

const (
	DIR_IMAGES = "images"
	DIR_AUDIO  = "audio"
	DIR_TMP    = "tmp"

	CLIP_TYPE_SCREEN = "SCREEN"
	CLIP_TYPE_AUDIO  = "AUDIO"
)

type Clip struct {
	Id             int64
	ImagesPath     string
	AudioPath      string
	TmpPath        string
	AudioDeviceNum int
	StopChannel    chan bool
}

type ClipType string

//TODO: extract db/mkdir to a service
func NewClip(db *sql.DB, clipType ClipType) *Clip {
	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(fmt.Sprintf("INSERT INTO clip (clip_type) VALUES ('%s')", clipType)) //TODO bind?
	if err != nil {
		panic(err)
	}

	rows, err := db.Query("SELECT last_insert_rowid()")
	if err != nil {
		panic(err)
	}

	defer rows.Close()

	if rows.Next() {
		var id int64
		err = rows.Scan(&id)
		if err != nil {
			panic(err)
		}
		tx.Commit()

		clip := &Clip{
			id,
			filepath.Join(fmt.Sprintf("clip%d", id), DIR_IMAGES),
			filepath.Join(fmt.Sprintf("clip%d", id), DIR_AUDIO),
			filepath.Join(fmt.Sprintf("clip%d", id), DIR_TMP),
			-1,
			make(chan bool, 1),
		}

		createDirectories(clip)

		return clip
	} else {
		panic("No rows returned") //TODO: error hanlding
	}
}

func createDirectories(clip *Clip) {
	utils.HandleError(os.MkdirAll(clip.ImagesPath, os.ModePerm))
	utils.HandleError(os.MkdirAll(clip.AudioPath, os.ModePerm))
	utils.HandleError(os.MkdirAll(clip.TmpPath, os.ModePerm))
	//TODO: handler errors, return err
}
