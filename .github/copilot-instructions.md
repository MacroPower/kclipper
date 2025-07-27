# Instructions

## Code Style

For public APIs, generally prefer to use a `struct` with `func` methods. Do your best to name structs using action verbs in their base form, e.g. `Add`, `Compute`, `Process`. We document all public APIs with Go Doc comments. For example:

```go
// Add is a simple adder.
type Add struct{}

// NewAdd creates a new [Add].
func NewAdd() *Add {
	return &Add{}
}

// Add adds two integers and returns the result.
func (a *Add) Add(x, y int) int {
	return x + y
}
```

## Error Handling

We wrap all errors with `fmt.Errorf` to add context. We use global error variables for common errors. For example:

```go
var ErrNotFound = errors.New("resource not found")

// ...

if err != nil {
	return fmt.Errorf("%w: %w", ErrNotFound, err)
}
```

For this reason, keep error messages short and to the point, since they may be wrapped many times. They should not contain the words `failed` or `error` except in the root context. This is important to reduce redundancy in the final error message. Some examples of good error messages are:

- `chart add failed: get "foo": resource not found`
- `chart update failed: generate schema "foo": parse "schema.json": invalid token at line 1:1`

When combining multiple errors, we use `github.com/hashicorp/go-multierror`. For example:

```go
var merr error

if err1 != nil {
	merr = multierror.Append(merr, err1)
}
if err2 != nil {
	merr = multierror.Append(merr, err2)
}

if merr != nil {
	return merr
}

return nil
```

## Testing

We always write tests in their own `_test` package. This means that only the public API can be tested.

We use `github.com/stretchr/testify/assert` and `github.com/stretchr/testify/require` for assertions.

We check the type of errors using `assert.ErrorIs`, which can be used to check against our global error variables.

We use table-driven tests where possible. For example:

```go
tcs := map[string]struct {
	input string
	want  string
}{
	"test case one": {
		input: "foo",
		want: "bar",
	},
	"test case two": {
		input: "baz",
		want: "qux",
	},
}
for name, tc := range tcs {
	t.Run(name, func(t *testing.T) {
		t.Parallel()
		got := someFunction(tc.input)
		assert.Equal(t, tc.want, got)
	})
}
```

We use Go 1.24, so you can use features from that version. Notably, you do not need to use `tc := tc` since Go 1.24 does not require it.

## Tools
- **Building**: When you need to test cross-compilation, run `devbox run -- task go-build`.
