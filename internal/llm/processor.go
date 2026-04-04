package llm

import (
	"context"

	"github.com/batkiz/rss-gateway/internal/model"
)

type Processor interface {
	Process(ctx context.Context, req model.ProcessRequest) (model.ProcessResponse, error)
}
