package auth

import "errors"

const (
	// ActionRead is a view of one domain or its children.
	ActionRead = "read"
	// ActionTrigger is run, session, version, experiment, deploy, or stop.
	ActionTrigger = "trigger"
	// ActionDelete removes the parent service, backtest, or strategy.
	ActionDelete = "delete"
	// ActionGrant adds or removes a grant. Owner is not a grant level.
	ActionGrant = "grant"
	// ActionCreate inserts a new top-level row owned by the caller.
	ActionCreate = "create"
)

var (
	ErrForbidden      = errors.New("forbidden")
	ErrNotFound       = errors.New("not found")
	ErrGrantNotFound  = errors.New("grant not found")
	ErrNotApproved    = errors.New("account is not approved")
	ErrOwnerGrant     = errors.New("owner is not a grant")
	ErrLoginAmbiguous = errors.New("login is ambiguous")
	ErrNameAmbiguous  = errors.New("name is ambiguous")
	ErrAlreadyExists  = errors.New("already exists")
	ErrBadRequest     = errors.New("bad request")
)
