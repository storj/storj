package put

import "context"

type PutContext interface {
	context.Context
	InnerContext() context.Context
	Recursive() bool
	Force() bool
	Prefix() string
	Delimiter() string
	WithPrefix(string) PutContext
}

type putContext struct {
	context.Context
	recursive, force bool
	prefix, delimiter string
}

func NewPutCtx(ctx context.Context, r, f bool, p, d string) *putContext {
	return &putContext{
		ctx,
		r,
		f,
		p,
		d,
	}
}

func NewPutCtxRecursive(ctx context.Context, r bool) *putContext {
	return NewPutCtx(ctx, r, false,"", "")
}

func NewPutCtxForce(ctx context.Context, f bool) *putContext {
	return NewPutCtx(ctx, false, f,"", "")
}

func NewPutCtxPrefix(ctx context.Context, p string) *putContext {
	return NewPutCtx(ctx, false, false, p, "")
}

func NewPutCtxDelimiter(ctx context.Context, d string) *putContext {
	return NewPutCtx(ctx, false, false,"", d)
}

func (c *putContext) InnerContext() context.Context {
	return c.Context
}

func (c *putContext) Recursive() bool {
	return c.recursive
}

func (c *putContext) Force() bool {
	return c.force
}

func (c *putContext) Prefix() string {
	return c.prefix
}

func (c *putContext) Delimiter() string {
	return c.delimiter
}

func (c *putContext) WithPrefix(p string) PutContext {
	return NewPutCtx(c.Context, c.recursive, c.force, p, c.delimiter)
}