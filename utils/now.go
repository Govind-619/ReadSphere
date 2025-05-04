package utils

import "time"

// NowUnix returns the current Unix timestamp as int64
func NowUnix() int64 {
    return time.Now().Unix()
}
