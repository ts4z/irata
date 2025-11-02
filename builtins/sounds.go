package builtins

import "github.com/ts4z/irata/soundmodel"

func SoundEffects() []*soundmodel.SoundEffect {
	return []*soundmodel.SoundEffect{
		{
			ID:          1,
			Name:        "School Bell",
			Description: "Ding, ding, ding",
			Path:        "/fs/SchoolBell.mp3",
		},
		{
			ID:          2,
			Name:        "Inchy",
			Description: "inchworm sound effect from Millipede",
			Path:        "/fs/inchy.mp3",
		},
		{
			ID:          3,
			Name:        "Sax A-D Fanfare",
			Description: "alto sax playing A-D fanfare",
			Path:        "/fs/alto-sax-a-d-fanfare.mp3",
		},
	}
}
