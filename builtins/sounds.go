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
			Name:        "Sax B♭-F Fanfare",
			Description: "sax choir playing fanfare",
			Path:        "/fs/sax-choir-Bb-F.wav",
		},
	}
}
