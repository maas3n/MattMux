package main

import "time"

// DVDChapter is chapter metadata returned by the external dvdvideo/libdvdread/libdvdnav path.
type DVDChapter struct {
	Number   int
	Start    time.Duration
	Duration time.Duration
}
