package utils

import (
	"time"
)

func HandleError(err error) {
	if err != nil {
		panic(err)
	}
}

func Fps(fps int) time.Duration {
	return time.Duration(1000/fps) * time.Millisecond
}

func Mills() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
