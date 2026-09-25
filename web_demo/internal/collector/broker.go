package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/capability"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"net/http"
)

type BrokerRecorder struct {
	Client                                       *http.Client
	SoftwareSHA256, ProfileSHA256, DatasetSHA256 string
}

func NewBrokerRecorder(socket, softwareSHA256, profileSHA256, datasetSHA256 string) (*BrokerRecorder, error) {
	if socket == "" || socket[0] != '/' || !digestRE.MatchString(softwareSHA256) || !digestRE.MatchString(profileSHA256) || !digestRE.MatchString(datasetSHA256) {
		return nil, errors.New("collector socket and pinned event revisions required")
	}
	return &BrokerRecorder{ipc.Client(socket), softwareSHA256, profileSHA256, datasetSHA256}, nil
}

type TrustedRecord struct {
	Claims         capability.Claims `json:"claims"`
	RequestID      string            `json:"request_id"`
	Kind           string            `json:"kind"`
	Payload        json.RawMessage   `json:"payload"`
	SoftwareSHA256 string            `json:"software_sha256"`
	ProfileSHA256  string            `json:"profile_sha256"`
	DatasetSHA256  string            `json:"dataset_sha256"`
}

func (r *BrokerRecorder) Record(ctx context.Context, c capability.Claims, requestID, kind string, payload map[string]any) error {
	raw, e := json.Marshal(payload)
	if e != nil {
		return ErrRejected
	}
	body, e := json.Marshal(TrustedRecord{c, requestID, kind, raw, r.SoftwareSHA256, r.ProfileSHA256, r.DatasetSHA256})
	if e != nil || len(body) > MaxEvent {
		return ErrRejected
	}
	req, e := http.NewRequestWithContext(ctx, "POST", "http://collector/v1/trusted-events", bytes.NewReader(body))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := r.Client.Do(req)
	if e != nil {
		return ErrBackpressure
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ErrBackpressure
	}
	return nil
}
