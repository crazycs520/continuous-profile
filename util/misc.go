package util

import "time"

func GetTimeStamp(t time.Time) int64 {
	return t.Unix()
}

// Millisecond returns the millisecond timestamp of the input time.
func Millisecond(t time.Time) int64 {
	return t.Unix()*1000 + int64(t.Nanosecond())/int64(time.Millisecond)
}

// GoWithRecovery wraps goroutine startup call with force recovery.
// it will dump current goroutine stack into log if catch any recover result.
//   exec:      execute logic function.
//   recoverFn: handler will be called after recover and before dump stack, passing `nil` means noop.
func GoWithRecovery(exec func(), recoverFn func(r interface{})) {
	defer func() {
		r := recover()
		if recoverFn != nil {
			recoverFn(r)
		}
		if r != nil {
			//logutil.BgLogger().Error("panic in the recoverable goroutine",
			//	zap.Reflect("r", r),
			//	zap.Stack("stack trace"))
		}
	}()
	exec()
}
