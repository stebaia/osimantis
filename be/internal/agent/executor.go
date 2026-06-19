package agent

import (
	"context"

	"relazioni-server/internal/tools"

	"github.com/jackc/pgx/v5/pgxpool"
)

// registryExecutor adatta tools.Registry + pool all'interfaccia ToolExecutor,
// fissando il pool così l'agente non deve conoscere il database.
type registryExecutor struct {
	registry tools.Registry
	pool     *pgxpool.Pool
}

// NewToolExecutor costruisce un ToolExecutor che esegue i tool del registry
// contro il pool dato.
func NewToolExecutor(registry tools.Registry, pool *pgxpool.Pool) ToolExecutor {
	return &registryExecutor{registry: registry, pool: pool}
}

func (e *registryExecutor) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	return e.registry.Call(ctx, e.pool, name, args)
}
