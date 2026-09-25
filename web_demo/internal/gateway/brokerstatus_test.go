package gateway

import (
	"strings"
	"testing"
)

func TestBrokerStatusShowsRunExposureAndFailsClosed(t *testing.T) {
	body := `{"usage":{"allocation_usd_micros":1000000,"settled_usd_micros":200001,"reserved_usd_micros":50000,"unknown_usd_micros":650000,"alert_percent":90,"paused":true},"active":2,"queued":4,"global_concurrency":3,"per_user_concurrency":1,"recording":true}`
	status := readBrokerStatus(strings.NewReader(body))
	if !status.Available || !status.Recording || !status.Paused || status.Spent != "$0.200001" || status.Unknown != "$0.650000" || status.AlertPercent != 90 || status.Active != 2 || status.Queued != 4 {
		t.Fatalf("incorrect accounting display: %#v", status)
	}
	for _, invalid := range []string{`{}`, body + `{}`, strings.Replace(body, `"global_concurrency":3`, `"global_concurrency":4`, 1), strings.Replace(body, `"settled_usd_micros":200001`, `"settled_usd_micros":-1`, 1), strings.Replace(body, `"recording":true`, `"recording":true,"private_token":"must not render"`, 1)} {
		if readBrokerStatus(strings.NewReader(invalid)).Available {
			t.Fatal("untrusted or partial status presented as available")
		}
	}
}
