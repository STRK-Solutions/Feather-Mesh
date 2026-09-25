package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/budget"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"io"
	"net/http"
	"time"
)

// BrokerStatus is aggregate run metadata only; it contains no conversation,
// credential, account history or authority to increase the operator ledger.
type BrokerStatus struct {
	Available    bool
	Recording    bool
	Paused       bool
	Allocated    string
	Spent        string
	Reserved     string
	Unknown      string
	AlertPercent int
	Active       int
	Queued       int
}

func (g *Gateway) brokerStatus(ctx context.Context) BrokerStatus {
	if g.Config.BrokerSocket == "" {
		return BrokerStatus{}
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", "http://broker/v1/status", nil)
	if e != nil {
		return BrokerStatus{}
	}
	response, e := ipc.Client(g.Config.BrokerSocket).Do(req)
	if e != nil {
		return BrokerStatus{}
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return BrokerStatus{}
	}
	return readBrokerStatus(response.Body)
}
func readBrokerStatus(source io.Reader) BrokerStatus {
	var in struct {
		Usage     budget.Usage `json:"usage"`
		Active    int          `json:"active"`
		Queued    int          `json:"queued"`
		Global    int          `json:"global_concurrency"`
		PerUser   int          `json:"per_user_concurrency"`
		Recording bool         `json:"recording"`
	}
	decoder := json.NewDecoder(io.LimitReader(source, 16<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || decoder.Decode(new(any)) != io.EOF {
		return BrokerStatus{}
	}
	u := in.Usage
	if in.Global != 3 || in.PerUser != 1 || in.Active < 0 || in.Active > 3 || in.Queued < 0 || in.Queued > 10000 || u.Amount < 1 || u.Amount > 1000000000000 || u.Settled < 0 || u.Reserved < 0 || u.Unknown < 0 || u.Settled > 1000000000000 || u.Reserved > u.Amount || u.Unknown > u.Amount || u.Alert != 0 && u.Alert != 75 && u.Alert != 90 {
		return BrokerStatus{}
	}
	dollars := func(n int64) string { return fmt.Sprintf("$%d.%06d", n/1000000, n%1000000) }
	return BrokerStatus{Available: true, Recording: in.Recording, Paused: u.Paused, Allocated: dollars(u.Amount), Spent: dollars(u.Settled), Reserved: dollars(u.Reserved), Unknown: dollars(u.Unknown), AlertPercent: u.Alert, Active: in.Active, Queued: in.Queued}
}
