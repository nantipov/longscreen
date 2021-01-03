package service

import (
	"database/sql"

	"github.com/nantipov/longscreen/internal/utils"
	"github.com/nantipov/longscreen/internal/utils/database"

	"github.com/gordonklaus/portaudio"

	"github.com/nantipov/longscreen/internal/domain"
)

var db *sql.DB
var globalSettings *domain.GlobalSettings = domain.InitSettings()
var audioDevices []*portaudio.DeviceInfo

func InitResources() {
	initDb()
	initAudio()
}

func initDb() {
	var err error
	db, err = sql.Open("sqlite3", "./project.db")
	if err != nil {
		panic(err)
	}
	upgradeDB()
}

func initAudio() {
	portaudio.Initialize()
	audioDevicesAll, err := portaudio.Devices()
	utils.HandleError(err)
	audioDevices = make([]*portaudio.DeviceInfo, 0)
	for _, d := range audioDevicesAll {
		if d.MaxInputChannels > 0 {
			audioDevices = append(audioDevices, d)
		}
	}
}

func GetDatabase() *sql.DB {
	return db
}

func GetGlobalSettings() *domain.GlobalSettings {
	return globalSettings
}

func GetAudioDevices() []*portaudio.DeviceInfo {
	return audioDevices
}

func FinalizeResources() {
	portaudio.Terminate()
	db.Close()
}

func upgradeDB() {
	//TODO: migration scripts
	// sqlStmt := `
	// CREATE TABLE IF NOT EXISTS clip (
	// 	id INTEGER PRIMARY KEY AUTOINCREMENT
	// )
	// `
	// _, err := db.Exec(sqlStmt)
	// if err != nil {
	// 	panic(err)
	// }
	database.ApplyMigrations(GetDatabase())
}
