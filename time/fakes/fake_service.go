package fakes

import "time"

type FakeService struct {
	NowTime       time.Time
	SleepDuration time.Duration
}

func (f *FakeService) Now() time.Time {
	return f.NowTime
}

func (f *FakeService) Sleep(duration time.Duration) {
	f.SleepDuration = duration
}
