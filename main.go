package main

import (
	"bufio"
	"strconv"

	"github.com/nantipov/longscreen/internal/utils"

	"fmt"
	"image"
	"image/png"

	// "log"
	"strings"

	"os"

	"github.com/nantipov/longscreen/internal/domain"

	screenshot "github.com/4nte/screenshot"
	tm "github.com/buger/goterm"
	_ "github.com/mattn/go-sqlite3"
	screenshot1 "github.com/rostislaved/screenshot"

	"github.com/nantipov/longscreen/internal/service"
)

func main() {
	service.InitResources()
	defer service.FinalizeResources()
	openDialog()
}

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

func processCommand(inputText string) {
	if inputText == "screen" {
		clip := domain.NewClip(service.GetDatabase()) //TODO: pass db as parameter to processCommand()?
		service.GetGlobalSettings().AddClip(clip)
		go service.RecordScreen(clip)
		fmt.Printf("Started recording screen #%d\n", clip.Id)
	} else if strings.HasPrefix(inputText, "audio ") {
		params := strings.Split(inputText, " ") //TODO fmt.Scanf
		deviceNum, err := strconv.Atoi(params[1])
		utils.HandleError(err)
		clip := domain.NewClip(service.GetDatabase()) //TODO: pass db as parameter to processCommand()?
		clip.AudioDeviceNum = deviceNum
		service.GetGlobalSettings().SetSpeed(domain.RECORDER_SPEED_REALTIME)
		service.GetGlobalSettings().AddClip(clip)
		go service.RecordAudio(clip)
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
	}
}

func stopClipById(id int64) {
	clip := service.GetGlobalSettings().GetClipById(id)
	if clip != nil {
		go func() { clip.StopChannel <- true }()
		fmt.Printf("Clip #%d is being stopped\n", id)
	}
}

func main0() {
	fmt.Print("Hey!\n")

	// db, err := sql.Open("sqlite3", "./session.db")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer db.Close()

	// n := screenshot.NumActiveDisplays()

	// for i := 0; i < n; i++ {
	// 	bounds := screenshot.GetDisplayBounds(i)

	// 	img, err := screenshot.CaptureRect(bounds)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	fileName := fmt.Sprintf("%d_%dx%d.png", i, bounds.Dx(), bounds.Dy())
	// 	file, _ := os.Create(fileName)
	// 	defer file.Close()
	// 	png.Encode(file, img)

	// 	fmt.Printf("#%d : %v \"%s\"\n", i, bounds, fileName)
	// }

	tm.Println(tm.Color("Connecting and making a screenshot", tm.YELLOW))
	tm.Flush()

	screenshoter := screenshot1.New()

	img, err := screenshoter.CaptureScreen()
	if err != nil {
		panic(err)
	}
	// myImg := image.Image(img)
	tm.Println(tm.Color("Saving file onto disk", tm.CYAN))
	tm.Flush()
	save(img, "hey.png")
	takeScreen()

}

func takeScreen() {
	return
	// Capture each displays.

	xgbConn, err := screenshot.NewXgbConnection()
	if err != nil {
		panic(err)
	}

	n := xgbConn.NumActiveDisplays()
	if n <= 0 {
		panic("Active display not found")
	}

	var all image.Rectangle = image.Rect(0, 0, 0, 0)

	for i := 0; i < n; i++ {
		bounds := xgbConn.GetDisplayBounds(i)
		all = bounds.Union(all)

		img, err := xgbConn.CaptureRect(bounds)
		if err != nil {
			panic(err)
		}
		fileName := fmt.Sprintf("%d_%dx%d.png", i, bounds.Dx(), bounds.Dy())
		save(img, fileName)

		//fmt.Printf("#%d : %v \"%s\"\n", i, bounds, fileName)
	}

	// Capture all desktop region into an image.
	fmt.Printf("%v\n", all)
	_, err = xgbConn.Capture(all.Min.X, all.Min.Y, all.Dx(), all.Dy())
	if err != nil {
		panic(err)
	}
	//save(img, "all.png")
}

func save(img *image.RGBA, filePath string) {
	file, err := os.Create(filePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	png.Encode(file, img)
}

/*

package main

import (
    "bufio"
    "fmt"
    "os"
)


// Three ways of taking input
//   1. fmt.Scanln(&input)
//   2. reader.ReadString()
//   3. scanner.Scan()
//
//  Here we recommend using bufio.NewScanner


func main() {
    // To create dynamic array
    arr := make([]string, 0)
    scanner := bufio.NewScanner(os.Stdin)
    for {
        fmt.Print("Enter Text: ")
        // Scans a line from Stdin(Console)
        scanner.Scan()
        // Holds the string that scanned
        text := scanner.Text()
        if len(text) != 0 {
            fmt.Println(text)
            arr = append(arr, text)
        } else {
            break
        }

    }
    // Use collected inputs
    fmt.Println(arr)
}

*/
