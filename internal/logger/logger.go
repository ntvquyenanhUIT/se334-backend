package logger

import (
	"go.uber.org/zap"
)

var Log *zap.Logger

func InitLogger() {
	var err error
	Log, err = zap.NewDevelopment()
	if err != nil {
		panic("Failed to init logger: " + err.Error())
	}
}

func SyncLogger() {
	_ = Log.Sync()
}
