package history

import (
	"context"
)

// Service coordinates the history view. The repository does the heavy lifting;
// the service only applies default filter values.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// List returns the paginated history. Caller (handler) is responsible for
// applying the role-based member_id filter — the service trusts what it gets.
func (s *Service) List(ctx context.Context, f Filter) ([]Entry, int, error) {
	return s.repo.List(ctx, f)
}
