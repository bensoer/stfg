package internal

import "time"

func Contains(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

func ParseDate(d string) (time.Time, error) {
	return time.Parse(time.RFC3339, d)
}
