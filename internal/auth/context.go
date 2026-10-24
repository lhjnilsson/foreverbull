package auth

import "context"

type accountKey struct{}

// WithAccount stores the caller on ctx. Handlers read it with FromContext.
func WithAccount(ctx context.Context, account Account) context.Context {
	return context.WithValue(ctx, accountKey{}, account)
}

// FromContext returns the caller put there by the HTTP wrapper or the bearer interceptor.
func FromContext(ctx context.Context) (Account, bool) {
	account, ok := ctx.Value(accountKey{}).(Account)
	if !ok || account.ID == "" {
		return Account{}, false
	}
	return account, true
}
