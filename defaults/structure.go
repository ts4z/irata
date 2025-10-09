package defaults

import (
	"fmt"

	"github.com/ts4z/irata/model"
)

func makeBreak(desc string, durationMins int) *model.Level {
	return &model.Level{
		Description:     desc,
		DurationMinutes: durationMins, // todo: convert to a Duration (string).
		IsBreak:         true,
	}
}

func Structure() *model.Structure {
	n := 0
	makeLevel := func(desc string) *model.Level {
		n++
		return &model.Level{
			Banner:          fmt.Sprintf("LEVEL %d", n),
			Description:     desc,
			DurationMinutes: 15,
			IsBreak:         false,
		}
	}
	return &model.Structure{
		StructureData: model.StructureData{
			ChipsPerAddOn: 0,
			ChipsPerBuyIn: 1500,
			Levels: []*model.Level{
				makeBreak("AWAITING START...", 5),
				makeLevel("5-5 + 5 ANTE"),
				makeLevel("5-10 + 10 ANTE"),
				makeLevel("10-15 + 15 ANTE"),
				makeLevel("15-30 + 30 ANTE"),
				makeBreak("RACE OFF 5 CHIPS", 15),
				makeLevel("20-40 + 40 ANTE"),
				makeLevel("30-60 + 60 ANTE"),
				makeLevel("40-80 + 80 ANTE"),
				makeLevel("60-120 + 120 ANTE"),
				makeLevel("100-200 + 200 ANTE"),
				makeBreak("RACE OFF 20 CHIPS", 15),
			},
		},
	}
}
