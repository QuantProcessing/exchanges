package account

import "github.com/QuantProcessing/exchanges/model"

var orderStateTransitions = map[model.OrderStatus]map[model.OrderStatus]model.OrderStatus{
	model.OrderStatusInitialized: {
		model.OrderStatusDenied:    model.OrderStatusDenied,
		model.OrderStatusEmulated:  model.OrderStatusEmulated,
		model.OrderStatusReleased:  model.OrderStatusReleased,
		model.OrderStatusSubmitted: model.OrderStatusSubmitted,
		model.OrderStatusRejected:  model.OrderStatusRejected,
		model.OrderStatusAccepted:  model.OrderStatusAccepted,
		model.OrderStatusCanceled:  model.OrderStatusCanceled,
		model.OrderStatusExpired:   model.OrderStatusExpired,
		model.OrderStatusTriggered: model.OrderStatusTriggered,
	},
	model.OrderStatusEmulated: {
		model.OrderStatusCanceled: model.OrderStatusCanceled,
		model.OrderStatusExpired:  model.OrderStatusExpired,
		model.OrderStatusReleased: model.OrderStatusReleased,
	},
	model.OrderStatusReleased: {
		model.OrderStatusDenied:    model.OrderStatusDenied,
		model.OrderStatusSubmitted: model.OrderStatusSubmitted,
		model.OrderStatusCanceled:  model.OrderStatusCanceled,
	},
	model.OrderStatusSubmitted: {
		model.OrderStatusPendingUpdate:   model.OrderStatusPendingUpdate,
		model.OrderStatusPendingCancel:   model.OrderStatusPendingCancel,
		model.OrderStatusRejected:        model.OrderStatusRejected,
		model.OrderStatusCanceled:        model.OrderStatusCanceled,
		model.OrderStatusAccepted:        model.OrderStatusAccepted,
		model.OrderStatusPartiallyFilled: model.OrderStatusPartiallyFilled,
		model.OrderStatusFilled:          model.OrderStatusFilled,
	},
	model.OrderStatusAccepted: {
		model.OrderStatusRejected:        model.OrderStatusRejected,
		model.OrderStatusTriggered:       model.OrderStatusTriggered,
		model.OrderStatusPendingUpdate:   model.OrderStatusPendingUpdate,
		model.OrderStatusPendingCancel:   model.OrderStatusPendingCancel,
		model.OrderStatusPartiallyFilled: model.OrderStatusPartiallyFilled,
		model.OrderStatusFilled:          model.OrderStatusFilled,
		model.OrderStatusCanceled:        model.OrderStatusCanceled,
		model.OrderStatusExpired:         model.OrderStatusExpired,
	},
	model.OrderStatusCanceled: {
		model.OrderStatusPartiallyFilled: model.OrderStatusPartiallyFilled,
		model.OrderStatusFilled:          model.OrderStatusFilled,
	},
	model.OrderStatusTriggered: {
		model.OrderStatusRejected:        model.OrderStatusRejected,
		model.OrderStatusPendingUpdate:   model.OrderStatusPendingUpdate,
		model.OrderStatusPendingCancel:   model.OrderStatusPendingCancel,
		model.OrderStatusPartiallyFilled: model.OrderStatusPartiallyFilled,
		model.OrderStatusFilled:          model.OrderStatusFilled,
		model.OrderStatusCanceled:        model.OrderStatusCanceled,
		model.OrderStatusExpired:         model.OrderStatusExpired,
	},
	model.OrderStatusPendingUpdate: {
		model.OrderStatusRejected:        model.OrderStatusRejected,
		model.OrderStatusAccepted:        model.OrderStatusAccepted,
		model.OrderStatusCanceled:        model.OrderStatusCanceled,
		model.OrderStatusExpired:         model.OrderStatusExpired,
		model.OrderStatusTriggered:       model.OrderStatusTriggered,
		model.OrderStatusSubmitted:       model.OrderStatusPendingUpdate,
		model.OrderStatusPendingUpdate:   model.OrderStatusPendingUpdate,
		model.OrderStatusPendingCancel:   model.OrderStatusPendingCancel,
		model.OrderStatusPartiallyFilled: model.OrderStatusPartiallyFilled,
		model.OrderStatusFilled:          model.OrderStatusFilled,
	},
	model.OrderStatusPendingCancel: {
		model.OrderStatusRejected:        model.OrderStatusRejected,
		model.OrderStatusPendingCancel:   model.OrderStatusPendingCancel,
		model.OrderStatusCanceled:        model.OrderStatusCanceled,
		model.OrderStatusExpired:         model.OrderStatusExpired,
		model.OrderStatusAccepted:        model.OrderStatusAccepted,
		model.OrderStatusPartiallyFilled: model.OrderStatusPartiallyFilled,
		model.OrderStatusFilled:          model.OrderStatusFilled,
	},
	model.OrderStatusPartiallyFilled: {
		model.OrderStatusPendingUpdate:   model.OrderStatusPendingUpdate,
		model.OrderStatusPendingCancel:   model.OrderStatusPendingCancel,
		model.OrderStatusFilled:          model.OrderStatusFilled,
		model.OrderStatusCanceled:        model.OrderStatusCanceled,
		model.OrderStatusExpired:         model.OrderStatusExpired,
		model.OrderStatusPartiallyFilled: model.OrderStatusPartiallyFilled,
	},
}

func CanOrderTransition(from model.OrderStatus, to model.OrderStatus) bool {
	_, ok := NextOrderStatus(from, to)
	return ok
}

func NextOrderStatus(from model.OrderStatus, to model.OrderStatus) (model.OrderStatus, bool) {
	if from == to {
		return to, true
	}
	if to == "" {
		return "", false
	}
	if from == "" {
		return to, true
	}
	targets, ok := orderStateTransitions[from]
	if !ok {
		return "", false
	}
	next, ok := targets[to]
	return next, ok
}
