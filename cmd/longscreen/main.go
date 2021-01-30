package main

import (
	"bufio"
	"runtime"
	"runtime/pprof"
	"strconv"

	"github.com/nantipov/longscreen/internal/service/exporter"

	"github.com/nantipov/longscreen/internal/utils"

	"fmt"

	// "log"
	"strings"

	"os"

	"github.com/nantipov/longscreen/internal/domain"

	//tm "github.com/buger/goterm"
	_ "github.com/mattn/go-sqlite3"

	"github.com/nantipov/longscreen/internal/service"
	"github.com/nantipov/longscreen/internal/service/recorder"
)

func main() {

	f, err := os.Create("cpu.prof")
	utils.HandleError(err)
	defer f.Close() // error handling omitted for example
	err = pprof.StartCPUProfile(f)
	utils.HandleError(err)
	defer pprof.StopCPUProfile()

	service.InitResources()
	defer service.FinalizeResources()
	openDialog()
}

func mem() {
	f, err := os.Create("mem.prof")
	utils.HandleError(err)

	runtime.GC() // get up-to-date statistics
	err = pprof.WriteHeapProfile(f)
	utils.HandleError(err)

	defer f.Close() // error handling omitted for example
}

// TODO move to the command/dialog service
func openDialog() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		scanner.Scan()
		text := scanner.Text()

		if strings.ToLower(text) == "exit" {
			break
		} else {
			processCommand(text)
		}
	}
}

// TODO move to the command/dialog service
func processCommand(inputText string) {
	if inputText == "screen" {
		clip := domain.NewClip(service.GetDatabase(), domain.CLIP_TYPE_SCREEN) //TODO: pass db as parameter to processCommand()?
		service.GetGlobalSettings().AddClip(clip)
		go recorder.RecordScreen(clip)
		fmt.Printf("Started recording screen #%d\n", clip.Id)
	} else if strings.HasPrefix(inputText, "audio ") {
		params := strings.Split(inputText, " ") //TODO fmt.Scanf
		deviceNum, err := strconv.Atoi(params[1])
		utils.HandleError(err)
		clip := domain.NewClip(service.GetDatabase(), domain.CLIP_TYPE_AUDIO) //TODO: pass db as parameter to processCommand()?
		clip.AudioDeviceNum = deviceNum
		service.GetGlobalSettings().SetSpeed(domain.RECORDER_SPEED_REALTIME)
		service.GetGlobalSettings().AddClip(clip)
		go recorder.RecordAudio(clip)
		fmt.Printf("Started recording audio #%d\n", clip.Id)
	} else if inputText == "stop" {
		maxId := service.GetGlobalSettings().GetMaxClipId()
		if maxId > 0 {
			stopClipById(maxId)
		}
		fmt.Println()
		service.GetGlobalSettings().SetSpeed(domain.RECORDER_SPEED_MIDDLE) //TODO restore speed after all audio clips are completed
	} else if inputText == "stop all" {
		for _, clipId := range service.GetGlobalSettings().GetAllClipIds() {
			stopClipById(clipId)
		}
		fmt.Println()
		service.GetGlobalSettings().SetSpeed(domain.RECORDER_SPEED_MIDDLE)
	} else if inputText == "devices" {
		for i, device := range service.GetAudioDevices() {
			fmt.Printf("[%d] %s\n", i, device.Name)
		}
	} else if inputText == "export" {
		exporter.Export()
	} else if inputText == "mem" {
		mem()
	}
}

func stopClipById(id int64) {
	clip := service.GetGlobalSettings().GetClipById(id)
	if clip != nil {
		go func() { clip.StopChannel <- true }()
		fmt.Printf("Clip #%d is being stopped\n", id)
	}
}
