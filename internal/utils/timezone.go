package utils

import "time"

// GetTaiwanTimezone returns the Taiwan timezone location.
// If loading fails, it returns a fixed zone UTC+8.
func GetTaiwanTimezone() *time.Location {
	taipeiTZ, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		// Fallback to fixed zone UTC+8
		taipeiTZ = time.FixedZone("CST", 8*3600)
	}
	return taipeiTZ
}

// NowInTaiwan returns the current time in Taiwan timezone.
func NowInTaiwan() time.Time {
	return time.Now().In(GetTaiwanTimezone())
}

// ToTaiwan converts a time to Taiwan timezone.
func ToTaiwan(t time.Time) time.Time {
	return t.In(GetTaiwanTimezone())
}