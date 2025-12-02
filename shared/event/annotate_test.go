// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package event

import (
	"context"
	"testing"
	"time"
)

func TestAnnotations_SetGet(t *testing.T) {
	a := &Annotations{kv: make(map[string]interface{})}
	a.Set("foo", 42)
	if got := a.Get("foo"); got != 42 {
		t.Errorf("expected 42, got %v", got)
	}
}

func TestAnnotations_Copy(t *testing.T) {
	a := &Annotations{kv: map[string]interface{}{"x": 1}}
	b := a.Copy()
	b.Set("x", 2)
	if a.Get("x") == b.Get("x") {
		t.Errorf("copy did not create a deep copy")
	}
}

func TestStartEvent_Annotate(t *testing.T) {
	ctx, done := StartEvent(context.Background(), "test")
	defer done()
	Annotate(ctx, "key", "val")
	val := ctx.Value(annotationKey).(*Annotations).Get("key")
	if val != "val" {
		t.Errorf("expected 'val', got %v", val)
	}
}

func TestForkEvent(t *testing.T) {
	ctx, _ := StartEvent(context.Background(), "parent")
	Annotate(ctx, "foo", "bar")
	child, _ := ForkEvent(ctx)
	Annotate(child, "foo", "baz")
	if ctx.Value(annotationKey).(*Annotations).Get("foo") == child.Value(annotationKey).(*Annotations).Get("foo") {
		t.Errorf("fork did not create a copy")
	}
}

func TestAnnotate_Types(t *testing.T) {
	ctx, _ := StartEvent(context.Background(), "types")
	Annotate(ctx, "int", 1)
	Annotate(ctx, "int64", int64(1))
	Annotate(ctx, "string", "foo")
	Annotate(ctx, "float", 1.5)
	Annotate(ctx, "bool", true)
	Annotate(ctx, "time", time.Now())
	Annotate(ctx, "duration", time.Second)
}
