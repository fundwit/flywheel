package common

import (
	"github.com/sirupsen/logrus"
	"os"
)

var Log *logrus.Logger

func init() {
	Log = logrus.New()
	Log.Out = os.Stdout
	Log.Formatter = &logrus.JSONFormatter{}
	Log.AddHook(&DefaultFieldsHook{})
}

type DefaultFieldsHook struct {
}

func (hook *DefaultFieldsHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook *DefaultFieldsHook) Fire(e *logrus.Entry) error {
	e.Data["serviceName"] = GetServiceName()
	e.Data["serviceInstance"] = GetServiceInstance()
	return nil
}
