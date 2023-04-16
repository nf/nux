package varvara

import "time"

type Datetime struct{}

func (Datetime) In(p byte) byte {
	t := time.Now()
	switch p {
	case 0x0:
		return byte(t.Year() >> 8)
	case 0x1:
		return byte(t.Year())
	case 0x2:
		return byte(t.Month()) - 1 // January is 0
	case 0x3:
		return byte(t.Day())
	case 0x4:
		return byte(t.Hour())
	case 0x5:
		return byte(t.Minute())
	case 0x6:
		return byte(t.Second())
	case 0x7:
		return byte(t.Weekday()) // Sunday is 0
	case 0x8:
		return byte((t.YearDay() - 1) >> 8) // 1 January is 0
	case 0x9:
		return byte(t.YearDay() - 1)
	case 0xa:
		if t.IsDST() {
			return 1
		} else {
			return 0
		}
	default:
		return 0
	}
}

func (Datetime) Out(p, b byte) {}
