// Package paramclient reaches a published PartSpec corpus over the contract's PartSpecService, as a
// param.Fetcher (agni issue 749). It is the transport adapter only; the caching, the generation
// check and the rule that an unreachable corpus is an error rather than an unseeded part all live in
// param.Remote, which wraps it.
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

// New returns a Client for the PartSpecService served at baseURL.
func New(baseURL string) *Client {
	return &Client{rpc: paramconnect.NewPartSpecServiceClient(&http.Client{Timeout: timeout}, baseURL)}
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
