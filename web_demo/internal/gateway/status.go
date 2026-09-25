package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
)

// Status is display-only and queried after local owner authorization. Neither
// controller state nor its absence grants browser or terminal access.
func (g *Gateway) workspaceStatus(ctx context.Context, workspace control.Workspace) (string, string, string, string) {
	if g.Config.ControllerSocket == "" {
		return "Status unavailable", "", "", ""
	}
	req, e := http.NewRequestWithContext(ctx, "GET", "http://controller/v1/workspaces/"+workspace.ID, nil)
	if e != nil {
		return "Status unavailable", "", "", ""
	}
	resp, e := ipc.Client(g.Config.ControllerSocket).Do(req)
	if e != nil {
		return "Status unavailable", "", "", ""
	}
	defer resp.Body.Close()
	var status struct {
		ID                   string `json:"id"`
		OwnerID              string `json:"owner_id"`
		DeploymentID         string `json:"deployment_id"`
		Generation           int64  `json:"generation"`
		AssignmentVersion    int64  `json:"assignment_version"`
		State                string `json:"state"`
		WarningAt            string `json:"warning_at"`
		RetentionNotified    string `json:"retention_notified"`
		RetentionDeleteAfter string `json:"retention_delete_after"`
	}
	d := json.NewDecoder(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != 200 || d.Decode(&status) != nil || d.Decode(new(any)) != io.EOF {
		return "Status unavailable", "", "", ""
	}
	if status.ID != workspace.ID || status.OwnerID != workspace.OwnerID || status.DeploymentID != workspace.DeploymentID || status.Generation != workspace.Generation || status.AssignmentVersion != workspace.GrantVersion {
		return "Updating workspace", "", "", ""
	}
	label := map[string]string{"running": "Running", "stopped": "Stopped", "starting": "Starting", "stopping": "Stopping", "deleted": "Deleted", "error": "Needs operator attention"}[status.State]
	if label == "" {
		return "Updating workspace", "", "", ""
	}
	warning := ""
	if status.State == "running" && status.WarningAt != "" {
		if _, e = time.Parse(time.RFC3339Nano, status.WarningAt); e == nil {
			warning = status.WarningAt
		}
	}
	notified, deleteAfter := "", ""
	n, ne := time.Parse(time.RFC3339Nano, status.RetentionNotified)
	deadline, de := time.Parse(time.RFC3339Nano, status.RetentionDeleteAfter)
	if ne == nil && de == nil && deadline.After(n) {
		notified, deleteAfter = n.UTC().Format(time.RFC3339), deadline.UTC().Format(time.RFC3339)
	}
	return label, warning, notified, deleteAfter
}
