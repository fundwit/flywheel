package common

import (
	"github.com/sirupsen/logrus"
	"os"
)

func init() {
	logger := logrus.StandardLogger()
	logger.Out = os.Stdout
	logger.Formatter = &logrus.JSONFormatter{}
	logger.AddHook(&DefaultFieldsHook{})
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
