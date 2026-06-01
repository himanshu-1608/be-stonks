package timeutil

import "time"

// IST is the India Standard Time zone (UTC+5:30).
var IST = time.FixedZone("IST", 5*3600+30*60)

// NowIST returns the current time in IST.
func NowIST() time.Time { return time.Now().In(IST) }

// FileStamp formats a time for use in filenames: 2006-01-02-15-04-05.
func FileStamp(t time.Time) string { return t.Format("2006-01-02-15-04-05") }

// Label formats a human/report-friendly timestamp: 2006-01-02-15-04-05 IST.
func Label(t time.Time) string { return t.Format("2006-01-02-15-04-05") + " IST" }
