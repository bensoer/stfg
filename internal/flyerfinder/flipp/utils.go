package flipp

import (
	"strconv"
	"time"
)

func generateSID() string {
	return strconv.FormatInt(time.Now().UnixMilli(), 10)
}
