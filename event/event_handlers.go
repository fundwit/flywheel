package event

import (
	"github.com/sirupsen/logrus"
)

/*
return nil if not support
*/
type EventHandler func(e *EventRecord) *EventHandleResult

type EventHandleResult struct {
	Success           bool
	Message           string
	HandlerIdentifier string
}

var EventHandlers []EventHandler

var InvokeHandlersFunc = invokeHandlers

func invokeHandlers(record *EventRecord) []EventHandleResult {
	results := []EventHandleResult{}
	for _, handler := range EventHandlers {
		logrus.Debug("pre handle event ", record.Event)
		r := handler(record)

		if r == nil {
			continue
		}

		results = append(results, *r)

		if r.Success {
			logrus.Info("post handle event. ", r)
		} else {
			logrus.Error("post handler error. ", r)
		}
	}
	return results
}
