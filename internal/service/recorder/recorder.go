package recorder

import (
	"database/sql"
	"fmt"
	"runtime/debug"

	"github.com/nantipov/longscreen/internal/domain"
	"github.com/nantipov/longscreen/internal/service"
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

func markClipAsStopped(clipId int64, db *sql.DB) {
	service.GetGlobalSettings().RemoveClipById(clipId)
	fmt.Printf("Clip #%d has been stopped\n", clipId)
	db.Exec(fmt.Sprintf("UPDATE clip SET is_stopped = 1 WHERE id = %d", clipId))
	debug.FreeOSMemory()
}
