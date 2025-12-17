package utils

import "sync/atomic"

type IDProvider struct {
	nextID int64
}

func NewIDProvider() *IDProvider {
	return &IDProvider{}
}

func (idp *IDProvider) GetID() int64 {
	return atomic.AddInt64(&idp.nextID, 1)
}
