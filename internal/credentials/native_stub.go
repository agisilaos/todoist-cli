//go:build !darwin || !cgo

package credentials

import "context"

type unavailable struct{}

func NewNative() Secrets                                         { return unavailable{} }
func (unavailable) Read(context.Context, string) (string, error) { return "", failure(Unavailable) }
func (unavailable) Write(context.Context, string, string) error  { return failure(Unavailable) }
func (unavailable) Delete(context.Context, string) error         { return failure(Unavailable) }
func (unavailable) Probe(context.Context) error                  { return failure(Unavailable) }
