// Package pipeline owns private dataset preparation and complete release promotion.
// FEAM remains the only authority that validates and writes provider manifests.
package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"strings"
	"time"
)

const Protocol = "feam.pipeline.v1"
const MiB int64 = 1024 * 1024

var ErrReconcile = errors.New("reconciliation_required")
var slug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var uuidPattern = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)

type Limits struct {
	SourceBytes    int64 `json:"source_bytes"`
	OutputBytes    int64 `json:"output_bytes"`
	CandidateBytes int64 `json:"candidate_bytes"`
	ElapsedSeconds int   `json:"elapsed_seconds"`
	Redirects      int   `json:"redirects"`
	ArchiveEntries int   `json:"archive_entries"`
	ExpandedBytes  int64 `json:"expanded_bytes"`
}

type Policy struct {
	Audience         string `json:"audience"`
	ModelMetadata    bool   `json:"model_metadata"`
	ModelPayload     bool   `json:"model_payload"`
	ResearchEligible bool   `json:"research_eligible"`
}

// ScientificTime records source science, never retrieval or publication time.
// Intervals are preserved through FEAM publication and STAC search/serialization.
type ScientificTime struct {
	Kind     string `json:"kind"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Evidence string `json:"evidence"`
}

type Source struct {
	ID              string         `json:"id"`
	Importer        string         `json:"importer"`
	URL             string         `json:"url"`
	SHA256          string         `json:"sha256"`
	MaxBytes        int64          `json:"max_bytes"`
	UpstreamVersion string         `json:"upstream_version"`
	RetrievedAt     string         `json:"retrieved_at"`
	License         string         `json:"license"`
	Attribution     string         `json:"attribution"`
	Station         string         `json:"station"`
	StartDate       string         `json:"start_date"`
	EndDate         string         `json:"end_date"`
	ScientificTime  ScientificTime `json:"scientific_time"`
	BBox            []float64      `json:"bbox"`
	Resampling      string         `json:"resampling"`
	Variable        string         `json:"variable"`
	Unit            string         `json:"unit"`
}

type Product struct {
	SourceID    string `json:"source_id"`
	ID          string `json:"id"`
	Version     string `json:"version"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IntendedUse string `json:"intended_use"`
	Limitations string `json:"limitations"`
	Producer    string `json:"producer"`
	Contact     string `json:"contact"`
	AssetID     string `json:"asset_id"`
}

type Job struct {
	Protocol        string      `json:"protocol"`
	ID              string      `json:"id"`
	Bundle          string      `json:"bundle"`
	Namespace       string      `json:"namespace"`
	ParentDigest    string      `json:"parent_digest"`
	ImporterVersion string      `json:"importer_version"`
	Sources         []Source    `json:"sources"`
	Products        []Product   `json:"products"`
	Limits          Limits      `json:"limits"`
	Policy          Policy      `json:"policy"`
	Withdrawal      *Withdrawal `json:"withdrawal,omitempty"`
}

type WithdrawalTarget struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}
type Withdrawal struct {
	Targets []WithdrawalTarget `json:"targets"`
	Reason  string             `json:"reason"`
}

func DecodeJob(data []byte) (Job, error) {
	var j Job
	if len(data) > 64*1024 {
		return j, errors.New("job exceeds 64 KiB")
	}
	// Reject duplicates as well as unknown fields: approvals bind one interpretation.
	if err := uniqueJSON(data); err != nil {
		return j, err
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return j, err
	}
	if err := requireFields(value, reflect.TypeFor[Job]()); err != nil {
		return j, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&j); err != nil {
		return j, errors.New("invalid closed job schema")
	}
	return j, j.Validate()
}

func requireFields(value any, kind reflect.Type) error {
	switch kind.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return errors.New("missing required job fields")
		}
		for index := 0; index < kind.NumField(); index++ {
			field := kind.Field(index)
			parts := strings.Split(field.Tag.Get("json"), ",")
			child, exists := object[parts[0]]
			if !exists && len(parts) > 1 && parts[1] == "omitempty" {
				continue
			}
			if !exists {
				return errors.New("missing required job field")
			}
			if err := requireFields(child, field.Type); err != nil {
				return err
			}
		}
	case reflect.Slice:
		items, ok := value.([]any)
		if !ok {
			return errors.New("job array required")
		}
		for _, item := range items {
			if err := requireFields(item, kind.Elem()); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		if value != nil {
			return requireFields(value, kind.Elem())
		}
	}
	return nil
}

func uniqueJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	var visit func(int) error
	visit = func(depth int) error {
		if depth > 16 {
			return errors.New("JSON nesting limit")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch t {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return e
				}
				k, ok := key.(string)
				if !ok || seen[k] {
					return errors.New("duplicate JSON key")
				}
				seen[k] = true
				if e = visit(depth + 1); e != nil {
					return e
				}
			}
			_, err = d.Token()
		case json.Delim('['):
			for d.More() {
				if err = visit(depth + 1); err != nil {
					return err
				}
			}
			_, err = d.Token()
		}
		return err
	}
	if err := visit(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}

func utc(s string) bool {
	t, err := time.Parse(time.RFC3339, s)
	return err == nil && strings.HasSuffix(s, "Z") && !t.IsZero()
}

func (j Job) Validate() error {
	bad := errors.New("invalid dataset job")
	if j.Withdrawal != nil {
		if j.ParentDigest == "" || len(j.Withdrawal.Targets) < 1 || len(j.Withdrawal.Targets) > 32 || strings.TrimSpace(j.Withdrawal.Reason) == "" || len(j.Withdrawal.Reason) > 2000 {
			return bad
		}
		seen := map[string]bool{}
		for _, t := range j.Withdrawal.Targets {
			key := t.ID + "/" + t.Version
			if !slug.MatchString(t.ID) || !slug.MatchString(t.Version) || seen[key] {
				return bad
			}
			seen[key] = true
		}
	}
	if j.Protocol != Protocol || !uuidPattern.MatchString(j.ID) || !slug.MatchString(j.Bundle) || j.Namespace != j.Bundle || (j.ParentDigest != "" && !digestPattern.MatchString(j.ParentDigest)) || j.ImporterVersion != "1" {
		return bad
	}
	if len(j.Sources) < 1 || len(j.Sources) > 2 || len(j.Products) != len(j.Sources) || j.Policy.Audience != "invited-users" || j.Policy.ModelPayload {
		return bad
	}
	l := j.Limits
	if l.SourceBytes < 1 || l.SourceBytes > 64*MiB || l.OutputBytes < 1 || l.OutputBytes > 250*MiB || l.CandidateBytes < l.OutputBytes || l.CandidateBytes > 250*MiB || l.ElapsedSeconds < 1 || l.ElapsedSeconds > 300 || l.Redirects < 0 || l.Redirects > 3 || l.ArchiveEntries != 0 || l.ExpandedBytes != 0 {
		return bad
	}
	seen := map[string]bool{}
	for _, s := range j.Sources {
		if s.MaxBytes < 1 || s.MaxBytes > j.Limits.SourceBytes {
			return bad
		}
		if !slug.MatchString(s.ID) || seen[s.ID] || !digestPattern.MatchString(s.SHA256) || !utc(s.RetrievedAt) || s.UpstreamVersion == "" || s.License != "https://open.canada.ca/en/open-government-licence-canada" || s.Attribution == "" || len(s.Attribution) > 2000 {
			return bad
		}
		seen[s.ID] = true
		if err := validateURL(s.Importer, s.URL); err != nil {
			return err
		}
		if s.Variable == "" || len(s.Variable) > 200 || s.Unit == "" || len(s.Unit) > 100 {
			return bad
		}
		if s.Importer == "eccc-csv" {
			a, e := time.Parse("2006-01-02", s.StartDate)
			b, e2 := time.Parse("2006-01-02", s.EndDate)
			if e != nil || e2 != nil || b.Before(a) || b.Sub(a) > 366*24*time.Hour || s.Station == "" || len(s.Station) > 32 || len(s.BBox) != 0 || s.Resampling != "none" || s.ScientificTime.Kind != "dates-in-source" || s.ScientificTime.Start != "" || s.ScientificTime.End != "" || s.ScientificTime.Evidence == "" {
				return bad
			}
		} else {
			st := s.ScientificTime
			if len(s.BBox) != 4 || s.BBox[0] < -180 || s.BBox[1] < -90 || s.BBox[2] > 180 || s.BBox[3] > 90 || s.BBox[0] >= s.BBox[2] || s.BBox[1] >= s.BBox[3] || s.Resampling != "nearest" || s.Station != "" || s.StartDate != "" || s.EndDate != "" || !utc(st.Start) || !utc(st.End) || st.End < st.Start || st.Evidence == "" || (st.Kind != "instant" && st.Kind != "climatology-interval") || (st.Kind == "instant" && st.Start != st.End) {
				return bad
			}
		}
	}
	products := map[string]bool{}
	sourceUse := map[string]bool{}
	for _, p := range j.Products {
		if !seen[p.SourceID] || sourceUse[p.SourceID] || !slug.MatchString(p.ID) || !slug.MatchString(p.Version) || !slug.MatchString(p.AssetID) || products[p.ID] {
			return bad
		}
		products[p.ID] = true
		sourceUse[p.SourceID] = true
		for _, v := range []string{p.Name, p.Description, p.IntendedUse, p.Limitations, p.Producer, p.Contact} {
			if v == "" || len(v) > 2000 {
				return bad
			}
		}
	}
	return nil
}

func validateURL(importer, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Opaque != "" || (u.Port() != "" && u.Port() != "443") || strings.Contains(u.EscapedPath(), "%") || strings.Contains(u.Path, "\\") || path.Clean(u.Path) != u.Path {
		return errors.New("unapproved source URL")
	}
	host := u.Hostname()
	allowed := importer == "eccc-csv" && host == "climate.weather.gc.ca" && u.Path == "/climate_data/bulk_data_e.html"
	allowed = allowed || (importer == "aafc-geotiff" && host == "agriculture.canada.ca" && strings.HasPrefix(u.Path, "/atlas/data_donnees/") && (strings.HasSuffix(strings.ToLower(u.Path), ".tif") || strings.HasSuffix(strings.ToLower(u.Path), ".tiff")) && u.RawQuery == "")
	if !allowed {
		return errors.New("unapproved source URL")
	}
	return nil
}

func (j Job) Hash() string {
	b, _ := json.Marshal(j)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
