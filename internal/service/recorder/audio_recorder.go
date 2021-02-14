package recorder

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"time"

	"github.com/nantipov/longscreen/internal/utils"

	"github.com/gordonklaus/portaudio"
	"github.com/nantipov/longscreen/internal/domain"
	"github.com/nantipov/longscreen/internal/service"
)

func RecordAudio(clip *domain.Clip) {
	//TODO minimalaudio https://github.com/gen2brain/malgo

	service.GetGlobalSettings().IncreaseAudioClipsCounter()

	db := service.GetDatabase()
	defer markClipAsStopped(clip.Id, db)
	defer service.GetGlobalSettings().DeacreaseAudioClipsCounter()

	device := service.GetAudioDevices()[clip.AudioDeviceNum]

	filename := "audio-0.aiff"
	f, err := os.Create(filepath.Join(clip.AudioPath, filename))
	utils.HandleError(err)

	// form chunk
	_, err = f.WriteString("FORM")
	utils.HandleError(err)
	utils.HandleError(binary.Write(f, binary.BigEndian, int32(0))) //total bytes
	_, err = f.WriteString("AIFF")
	utils.HandleError(err)

	// common chunk
	_, err = f.WriteString("COMM")
	utils.HandleError(err)
	utils.HandleError(binary.Write(f, binary.BigEndian, int32(18)))                      //size
	utils.HandleError(binary.Write(f, binary.BigEndian, int16(device.MaxInputChannels))) //channels
	utils.HandleError(binary.Write(f, binary.BigEndian, int32(0)))                       //number of samples
	utils.HandleError(binary.Write(f, binary.BigEndian, int16(32)))                      //bits per sample
	_, err = f.Write([]byte{0x40, 0x0e, 0xac, 0x44, 0, 0, 0, 0, 0, 0})                   //80-bit sample rate 44100
	utils.HandleError(err)

	// sound chunk
	_, err = f.WriteString("SSND")
	utils.HandleError(err)
	utils.HandleError(binary.Write(f, binary.BigEndian, int32(0))) //size
	utils.HandleError(binary.Write(f, binary.BigEndian, int32(0))) //offset
	utils.HandleError(binary.Write(f, binary.BigEndian, int32(0))) //block
	nSamples := 0
	defer func() {
		// fill in missing sizes
		totalBytes := 4 + 8 + 18 + 8 + 8 + 4*nSamples
		_, err = f.Seek(4, 0)
		utils.HandleError(err)
		utils.HandleError(binary.Write(f, binary.BigEndian, int32(totalBytes)))
		_, err = f.Seek(22, 0)
		utils.HandleError(err)
		utils.HandleError(binary.Write(f, binary.BigEndian, int32(nSamples)))
		_, err = f.Seek(42, 0)
		utils.HandleError(err)
		utils.HandleError(binary.Write(f, binary.BigEndian, int32(4*nSamples+8)))
		utils.HandleError(f.Close())
	}()

	in := make([]int32, 64)

	parameters := portaudio.HighLatencyParameters(device, nil)
	parameters.Input.Channels = device.MaxInputChannels
	parameters.Output.Channels = 0
	parameters.SampleRate = 44100
	parameters.FramesPerBuffer = len(in)
	stream, err := portaudio.OpenStream(parameters, in)
	utils.HandleError(err)
	defer stream.Close()

	utils.HandleError(stream.Start())
	ts0 := time.Now().Unix()
	for !isClipStopped(clip) {
		utils.HandleError(stream.Read())
		utils.HandleError(binary.Write(f, binary.BigEndian, in))
		nSamples += len(in)
	}
	ts1 := time.Now().Unix()
	utils.HandleError(stream.Stop())

	//TODO separate method
	// settings ////
	_, err = db.Exec("INSERT INTO audio_track (clip_id, ts0, ts1, seq) VALUES ($1, $2, $3, $4)", clip.Id, ts0, ts1, 0)
	utils.HandleError(err)
	////////////////
}
