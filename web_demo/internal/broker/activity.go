package broker

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
)

// Operations are bounded dispatch leases; queueing and polling never refresh
// workspace activity. The controller validates the broker's Unix peer identity.
type Operations interface {
	Set(context.Context, string, string, bool) error
}

type ControlOperations struct{ client *http.Client }

func NewControlOperations(socket string) ControlOperations {
	c := ipc.Client(socket)
	c.Timeout = 4 * time.Second
	return ControlOperations{c}
}

func (c ControlOperations) Set(ctx context.Context, workspace, request string, active bool) error {
	b, _ := json.Marshal(map[string]any{"workspace_id": workspace, "request_id": request, "active": active})
	r, err := http.NewRequestWithContext(ctx, "POST", "http://unix/v1/workspaces/operation", bytes.NewReader(b))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errOperation
	}
	return nil
}
