package domain

type (
	VideoTrack struct {
		Id     int64
		ClipId int64
		Width  int
		Height int
	}

	AudioTrack struct {
	}

	VideoFrame struct {
		Id     int64
		Seq    int
		MouseX int
		MouseY int
		Fps    int
		Ts     int64
	}
)
