// Package paramclient reaches a published PartSpec corpus over the contract's PartSpecService, as a
// param.Fetcher (agni issue 749). It is only the transport adapter. param.Remote wraps it and holds
// the caching, the generation check, and the rule that an unreachable corpus is an error rather
// than an unseeded part.
package paramclient

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/gen/go/agni/v1/param/paramconnect"
)

// timeout bounds one call. A design's batch is a few hundred part numbers, so a corpus that needs
// longer than this is better reported as unreachable than left to hang a check.
const timeout = 30 * time.Second

// Client is a param.Fetcher over a PartSpecService at a base URL, such as a running `agnids serve`.
type Client struct {
	rpc paramconnect.PartSpecServiceClient
}

// New returns a Client for the PartSpecService served at baseURL. A non-empty token goes on every
// call as a bearer token, for an agnids started with --auth tokens (agni issue 1765).
func New(baseURL, token string) *Client {
	var opts []connect.ClientOption
	if token != "" {
		opts = append(opts, connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set("Authorization", "Bearer "+token)
				return next(ctx, req)
			}
		})))
	}
	return &Client{rpc: paramconnect.NewPartSpecServiceClient(&http.Client{Timeout: timeout}, baseURL, opts...)}
}

// BatchGet implements param.Fetcher.
func (c *Client) BatchGet(ctx context.Context, mpns []string) ([]*parampb.PartSpec, uint64, error) {
	resp, err := c.rpc.BatchGetPartSpecs(ctx, connect.NewRequest(&parampb.BatchGetPartSpecsRequest{Mpns: mpns}))
	if err != nil {
		return nil, 0, err
	}
	return resp.Msg.GetSpecs(), resp.Msg.GetGeneration(), nil
}

// Generation implements param.Fetcher.
func (c *Client) Generation(ctx context.Context) (uint64, error) {
	resp, err := c.rpc.GetGeneration(ctx, connect.NewRequest(&parampb.GetGenerationRequest{}))
	if err != nil {
		return 0, err
	}
	return resp.Msg.GetGeneration(), nil
}
