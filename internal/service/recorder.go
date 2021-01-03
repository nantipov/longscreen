package service

import (
	"fmt"

	"github.com/nantipov/longscreen/internal/domain"
)

func isClipStopped(clip *domain.Clip) bool {
	select {
	case isStopped := <-clip.StopChannel:
		return isStopped
	default:
		return false
	}
	return false
}

func markClipAsStopped(clipId int64) {
	GetGlobalSettings().RemoveClipById(clipId)
	fmt.Printf("Clip #%d has been stopped\n", clipId)
}
