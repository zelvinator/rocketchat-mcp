package mcpserver

import "context"

// call runs fn and stops waiting when ctx is done.
//
// gorocket does not accept a context. The gorocket timeout ends the HTTP
// request after the handler returns.
func call[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	if err := ctx.Err(); err != nil {
		var zero T
		return zero, err
	}

	type result struct {
		value T
		err   error
	}

	done := make(chan result, 1)
	go func() {
		value, err := fn()
		done <- result{value: value, err: err}
	}()

	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case res := <-done:
		return res.value, res.err
	}
}
