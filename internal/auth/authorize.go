package auth

import "errors"

// MayCreate allows a new service, backtest, or strategy for a global editor or admin.
func MayCreate(account Account) error {
	if account.ID == "" {
		return errors.New("account id is empty")
	}
	if account.Role == RoleAdmin || account.Role == RoleEditor {
		return nil
	}
	return ErrForbidden
}

// Allow applies the relationship after the row is known to exist.
// visible is false when the caller cannot see the row.
// owner is true when owner_id is the caller.
// grantLevel is empty when the caller has no grant on the row.
func Allow(account Account, action, grantLevel string, owner, visible bool) error {
	if account.ID == "" {
		return errors.New("account id is empty")
	}
	if account.Role == RoleAdmin {
		return nil
	}
	if !visible {
		return ErrNotFound
	}
	switch action {
	case ActionRead:
		return nil
	case ActionTrigger:
		if owner && account.Role == RoleEditor {
			return nil
		}
		if grantLevel == LevelEditor && account.Role == RoleEditor {
			return nil
		}
		return ErrForbidden
	case ActionDelete:
		if owner && account.Role == RoleEditor {
			return nil
		}
		return ErrForbidden
	case ActionGrant:
		if owner && account.Role == RoleEditor {
			return nil
		}
		return ErrNotFound
	case ActionCreate:
		return MayCreate(account)
	default:
		return ErrForbidden
	}
}
