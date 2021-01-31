package domain

import (
	"sync"
)

type ScreenRecorderSpeed uint32

const (
	RECORDER_SPEED_RARE   ScreenRecorderSpeed = 1
	RECORDER_SPEED_MIDDLE ScreenRecorderSpeed = 2
	RECORDER_SPEED_OFTEN  ScreenRecorderSpeed = 3
)

type GlobalSettings struct {
	mu                *sync.RWMutex
	RecorderSpeed     ScreenRecorderSpeed
	activeClipsById   map[int64]*Clip
	audioClipsCounter int
}

func InitSettings() *GlobalSettings {
	return &GlobalSettings{
		mu:                &sync.RWMutex{},
		RecorderSpeed:     RECORDER_SPEED_MIDDLE,
		activeClipsById:   make(map[int64]*Clip),
		audioClipsCounter: 0,
	}
}

func (s *GlobalSettings) SetSpeed(speed ScreenRecorderSpeed) {
	s.mu.Lock()
	s.RecorderSpeed = speed
	s.mu.Unlock()
}

func (s *GlobalSettings) GetSpeed() ScreenRecorderSpeed {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.RecorderSpeed
}

func (s *GlobalSettings) AddClip(clip *Clip) {
	s.mu.Lock()
	s.activeClipsById[clip.Id] = clip
	s.mu.Unlock()
}

func (s *GlobalSettings) GetClipById(id int64) *Clip {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeClipsById[id]
}

func (s *GlobalSettings) GetMaxClipId() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var maxId int64 = -1
	for k, _ := range s.activeClipsById {
		if k > maxId {
			maxId = k
		}
	}
	return maxId
}

func (s *GlobalSettings) GetAllClipIds() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]int64, 0)
	for k, _ := range s.activeClipsById {
		ids = append(ids, k)
	}
	return ids
}

func (s *GlobalSettings) RemoveClipById(id int64) {
	s.mu.Lock()
	if len(s.activeClipsById) > 0 {
		delete(s.activeClipsById, id)
	}
	s.mu.Unlock()
}

func (s *GlobalSettings) IncreaseAudioClipsCounter() {
	s.mu.Lock()
	s.audioClipsCounter = s.audioClipsCounter + 1
	if s.audioClipsCounter > 0 {
		s.RecorderSpeed = RECORDER_SPEED_OFTEN //TODO reorganize logic domain-service
	}
	s.mu.Unlock()
}

func (s *GlobalSettings) DeacreaseAudioClipsCounter() {
	s.mu.Lock()
	s.audioClipsCounter = s.audioClipsCounter - 1
	if s.audioClipsCounter < 1 {
		s.RecorderSpeed = RECORDER_SPEED_MIDDLE //TODO reorganize logic domain-service
		s.audioClipsCounter = 0
	}
	s.mu.Unlock()
}
