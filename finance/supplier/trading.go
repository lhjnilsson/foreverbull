package supplier

import "github.com/lhjnilsson/foreverbull/finance/entity"

type Trading interface {
	GetPortfolio() (*entity.Portfolio, error)
	GetPositions() (*[]entity.Position, error)
	GetOrders() ([]*entity.Order, error)
}
