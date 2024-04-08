// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// Ball is the component registry.
type Ball struct {
	registry []*Component
}

// NewBall creates a new component registry.
func NewBall() *Ball {
	return &Ball{}
}

// getLogger returns with the zap Logger, i!f component is registered.
// used for internal logging.
func (ball *Ball) getLogger() *zap.Logger {
	if logger := lookup[*zap.Logger](ball); logger != nil {
		if logger.instance == nil {
			_ = logger.Init(context.Background())
		}
		if logger.instance != nil {
			return logger.instance.(*zap.Logger)
		}
	}
	return zap.NewNop()
}

// RegisterManual registers a component manually.
// Most of the time you need either Provide or View instead of this.
func RegisterManual[T any](
	ball *Ball,
	factory func(ctx context.Context) (T, error),
) {
	if component := lookup[T](ball); component != nil {
		panic("duplicate registration, " + name[T]())
	}
	component := &Component{}
	component.create = &Stage{
		run: func(a any, ctx context.Context) (err error) {
			component.instance, err = factory(ctx)
			return err
		},
	}

	component.target = reflect.TypeOf((*T)(nil)).Elem()
	ball.registry = append(ball.registry, component)
}

// Tag attaches a tag to the component registration.
func Tag[A any, Tag any](ball *Ball, tag Tag) {
	c := MustLookupComponent[A](ball)

	// we don't allow duplicated registrations, as we always return with the first value.
	for ix, existing := range c.tags {
		_, ok := existing.(Tag)
		if ok {
			c.tags[ix] = tag
			return
		}
	}
	c.tags = append(c.tags, tag)
}

// RemoveTag removes all the Tag type of tags from the component.
func RemoveTag[A any, Tag any](ball *Ball) {
	c := MustLookupComponent[A](ball)
	var filtered []any
	// we don't allow duplicated registrations, as we always return with the first value.
	for ix, existing := range c.tags {
		_, ok := existing.(Tag)
		if !ok {
			filtered = append(filtered, c.tags[ix])
		}
	}
	c.tags = filtered
}

// GetTag returns with attached tag (if attached).
func GetTag[A any, Tag any](ball *Ball) (Tag, bool) {
	c := MustLookupComponent[A](ball)
	return findTag[Tag](c)
}

// GetTagOf returns with attached tag (if attached).
func GetTagOf[Tag any](c *Component) (Tag, bool) {
	return findTag[Tag](c)
}

func findTag[Tag any](c *Component) (Tag, bool) {
	for _, tag := range c.tags {
		c, ok := tag.(Tag)
		if ok {
			return c, true
		}
	}
	var t Tag
	return t, false
}

// DependsOn creates a dependency relation between two components.
// With the help of the dependency graph, they can be executed in the right order.
func DependsOn[BASE any, DEPENDENCY any](ball *Ball) {
	c := MustLookupComponent[BASE](ball)
	c.addRequirement(typeOf[DEPENDENCY]())
}

// ForEach executes a callback action on all the selected components.
func ForEach(ball *Ball, callback func(component *Component) error, selectors ...ComponentSelector) error {
	return forEachComponent(sortedComponents(ball), callback, selectors...)
}

// ForEachDependency executes a callback action on all the components, matching the target selector and dependencies, but only if selectors parameter is matching them.
func ForEachDependency(ball *Ball, target ComponentSelector, callback func(component *Component) error, selectors ...ComponentSelector) error {
	return forEachComponent(FindSelectedWithDependencies(ball, target), callback, selectors...)
}

// Initialize components as ForEach callback.
func Initialize(ctx context.Context) func(c *Component) error {
	return func(c *Component) error {
		return c.Init(ctx)
	}
}

func forEachComponent(components []*Component, callback func(component *Component) error, selectors ...ComponentSelector) error {
	for _, c := range components {
		if len(selectors) == 0 {
			err := callback(c)
			if err != nil {
				return err
			}
		}
		for _, s := range selectors {
			if s(c) {
				err := callback(c)
				if err != nil {
					return err
				}
			}
			break
		}
	}
	return nil
}

// Execute executes a function with injecting all the required dependencies with type based Dependency Injection.
func Execute[A any](ctx context.Context, ball *Ball, factory interface{}) (A, error) {
	var a A
	response, err := runWithParams(ctx, ball, factory)
	if err != nil {
		return a, err
	}
	if len(response) > 1 {
		if response[1].Interface() != nil {
			return a, response[1].Interface().(error)
		}
	}
	if response[0].Interface() == nil {
		return a, errs.New("Provider factory is executed without error, but returner with nil instance. " + name[A]())
	}

	return response[0].Interface().(A), nil
}

// Execute0 executes a function with injection all required parameters. Same as Execute but without return type.
func Execute0(ctx context.Context, ball *Ball, factory interface{}) error {
	_, err := runWithParams(ctx, ball, factory)
	if err != nil {
		return err
	}
	return nil
}

// injectAnd execute calls the `factory` method, finding all required parameters in the registry.
func runWithParams(ctx context.Context, ball *Ball, factory interface{}) ([]reflect.Value, error) {
	ft := reflect.TypeOf(factory)
	if reflect.Func != ft.Kind() {
		panic("Provider argument must be a func()")
	}

	specificError := func(t reflect.Type, ix int, reason string) error {
		return errs.New("Couldn't inject %s to the %d argument of the provider function %v: %s", t, ix, reflect.ValueOf(factory).String(), reason)
	}

	var args []reflect.Value
	for i := 0; i < ft.NumIn(); i++ {
		// ball can be injected to anywhere. But it's better to not use.
		if ft.In(i) == reflect.TypeOf(ball) {
			args = append(args, reflect.ValueOf(ball))
			continue
		}

		// context can be injected without strong dependency
		if ctx != nil && ft.In(i) == typeOf[context.Context]() {
			args = append(args, reflect.ValueOf(ctx))
			continue
		}

		dep, ok := lookupByType(ball, ft.In(i))
		if ok {
			if dep.instance == nil {
				return nil, specificError(ft.In(i), i, "instance is nil (not yet initialized)")
			}
			args = append(args, reflect.ValueOf(dep.instance))
			continue
		}
		return nil, specificError(ft.In(i), i, "instance is not registered")

	}

	return reflect.ValueOf(factory).Call(args), nil
}

// Provide registers a new instance to the dependency pool.
// Run/Close methods are auto-detected (stage is created if they exist).
func Provide[A any](ball *Ball, factory interface{}) {
	RegisterManual[A](ball, func(ctx context.Context) (A, error) {
		return Execute[A](ctx, ball, factory)
	})

	t := typeOf[A]()
	component, _ := lookupByType(ball, t)

	// auto-detect Run method for Run stage
	runF, found := t.MethodByName("Run")
	if found {
		component.run = &Stage{
			background: true,
		}
		registerFunc[A](runF, component.run, "Run")
	}

	// auto-detect Close method for Close stage
	closeF, found := t.MethodByName("Close")
	if found {
		component.close = &Stage{}
		registerFunc[A](closeF, component.close, "Close")
	}

	// mark dependencies
	ft := reflect.TypeOf(factory)
	if ft.Kind() != reflect.Func {
		panic("factory parameter of Provide must be a func")
	}
	for i := 0; i < ft.NumIn(); i++ {
		// internal dependency without component
		if ft.In(i) == reflect.TypeOf(ball) {
			continue
		}

		// context can be injected any time
		if ft.In(i).String() == "context.Context" {
			continue
		}

		component.addRequirement(ft.In(i))
	}
}

// registerFunc tries to find a func with supported signature, to be used for stage runner func.
func registerFunc[A any](f reflect.Method, run *Stage, name string) {
	if !f.Func.IsValid() {
		return
	}
	switch runner := f.Func.Interface().(type) {
	case func(a A, ctx context.Context) error:
		run.run = func(a any, ctx context.Context) error {
			return runner(a.(A), ctx)
		}
	case func(a A) error:
		run.run = func(a any, ctx context.Context) error {
			return runner(a.(A))
		}
	default:
		panic(fmt.Sprintf("Unsupported %s method signature: %v", typeOf[A](), name))
	}
}

// Supply registers and instance which is already initialized.
func Supply[T any](ball *Ball, t T) {
	if lookup[T](ball) != nil {
		panic(fmt.Sprintf("Component instance is already provided with Supply: %v", typeOf[T]()))
	}
	if typeOf[T]().Kind() == reflect.Func {
		panic("function type for supply is not yet supported")
	}

	ball.registry = append(ball.registry, &Component{
		target:   typeOf[T](),
		instance: t,
		create: &Stage{
			started:  time.Now(),
			finished: time.Now(),
		},
	})
}

// View is lightweight component, which provides a type based on a existing instances.
func View[From any, To any](ball *Ball, convert func(From) To) {
	RegisterManual[To](ball, func(ctx context.Context) (To, error) {
		a := MustLookupComponent[From](ball)
		return convert(a.instance.(From)), nil
	})
	component := MustLookupComponent[To](ball)
	component.requirements = append(component.requirements, MustLookupComponent[From](ball).target)
}

// Dereference is a simple transformation to make real value from a pointer. Useful with View.
// for example: `View[*DB, DB](ball, Dereference[DB])`.
func Dereference[A any](a *A) A {
	return *a
}

func name[T any]() string {
	var a [0]T
	return reflect.TypeOf(a).Elem().String()
}

func lookup[T any](ball *Ball) *Component {
	var t [0]T
	tzpe := reflect.TypeOf(t).Elem()
	for _, c := range ball.registry {
		if c.target == tzpe {
			return c
		}
	}
	return nil
}

// MustLookupComponent gets the component (or panics if doesn't exist) based on a type.
func MustLookupComponent[T any](ball *Ball) *Component {
	c := lookup[T](ball)
	if c == nil {
		panic("component is missing: " + name[T]())
	}
	return c
}

func lookupByType(ball *Ball, tzpe reflect.Type) (*Component, bool) {
	for _, c := range ball.registry {
		if c.target == tzpe {
			return c, true
		}
	}
	return nil, false
}

func mustLookupByType(ball *Ball, tzpe reflect.Type) *Component {
	c, found := lookupByType(ball, tzpe)
	if !found {
		panic("component is missing: " + tzpe.String())
	}
	return c
}

// MustLookup returns with the registered component instance (or panic).
func MustLookup[T any](ball *Ball) T {
	component := MustLookupComponent[T](ball)
	if component.instance == nil {
		panic("lookup of an uninitialized component " + name[T]())
	}
	return component.instance.(T)
}

func typeOf[A any]() reflect.Type {
	var a [0]A
	return reflect.TypeOf(a).Elem()
}

func fullyQualifiedTypeName(t reflect.Type) string {
	if t.Kind() == reflect.Pointer {
		return "*" + fullyQualifiedTypeName(t.Elem())
	} else if t.Kind() == reflect.Slice {
		return "[]" + fullyQualifiedTypeName(t.Elem())
	}
	return t.PkgPath() + "." + t.Name()
}
