package event_test

import (
	"flywheel/event"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	. "github.com/onsi/gomega"
)

func TestInvokeHandlers(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should invoke all registered event handlers", func(t *testing.T) {
		event.EventHandlers = append(event.EventHandlers, func(e *event.EventRecord) *event.EventHandleResult {
			return nil
		})
		event.EventHandlers = append(event.EventHandlers, func(e *event.EventRecord) *event.EventHandleResult {
			return &event.EventHandleResult{Success: true, Message: "success", HandlerIdentifier: "all-success-handler"}
		})
		event.EventHandlers = append(event.EventHandlers, func(e *event.EventRecord) *event.EventHandleResult {
			return &event.EventHandleResult{Success: false, Message: "failure", HandlerIdentifier: "all-failure-handler"}
		})

		ev := event.EventRecord{
			Event: event.Event{
				SourceType: "WORK",
				SourceId:   1234,
				SourceDesc: "work1234",

				EventCategory: event.EventCategoryCreated,
				UpdatedProperties: event.UpdatedProperties{{PropertyName: "Name", PropertyDesc: "NameDesc",
					OldValue: "OldName", OldValueDesc: "OldNameDesc", NewValue: "NewName", NewValueDesc: "NewNameDesc"}},
				UpdatedRelations: event.UpdatedRelations{{PropertyName: "Address", PropertyDesc: "AddressDesc",
					TargetType: "address", TargetTypeDesc: "Address",
					OldTargetId: "addressOld", OldTargetDesc: "addressOldDesc", NewTargetId: "addressNew", NewTargetDesc: "addressNewDesc"}},

				CreatorId:   333,
				CreatorName: "user333",
			},
			Timestamp: types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
			Synced:    true,
		}

		ret := event.InvokeHandlersFunc(&ev)
		Expect(ret).To(Equal([]event.EventHandleResult{
			{Success: true, Message: "success", HandlerIdentifier: "all-success-handler"},
			{Success: false, Message: "failure", HandlerIdentifier: "all-failure-handler"},
		}))
	})
}
