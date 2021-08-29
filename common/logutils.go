package common

import (
	"os"

	"github.com/sirupsen/logrus"
)

func init() {
	logger := logrus.StandardLogger()
	logger.Out = os.Stdout
	logger.Formatter = &logrus.TextFormatter{}
	logger.AddHook(&DefaultFieldsHook{})
}

type DefaultFieldsHook struct {
}

func (hook *DefaultFieldsHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook *DefaultFieldsHook) Fire(e *logrus.Entry) error {
	// e.Data["serviceName"] = GetServiceName()
	// e.Data["serviceInstance"] = GetServiceInstance()
	return nil
}
