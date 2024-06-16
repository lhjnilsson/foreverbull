package supplier

import "github.com/lhjnilsson/foreverbull/pkg/finance/entity"

type Trading interface {
	GetPortfolio() (*entity.Portfolio, error)
	GetOrders() ([]*entity.Order, error)
}
