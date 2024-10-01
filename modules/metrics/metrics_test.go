package metrics

import (
	"testing"

	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
)

func TestMetricsUnmarshalKenginefile(t *testing.T) {
	m := &Metrics{}
	d := kenginefile.NewTestDispenser(`metrics bogus`)
	err := m.UnmarshalKenginefile(d)
	if err == nil {
		t.Errorf("expected error")
	}

	m = &Metrics{}
	d = kenginefile.NewTestDispenser(`metrics`)
	err = m.UnmarshalKenginefile(d)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if m.DisableOpenMetrics {
		t.Errorf("DisableOpenMetrics should've been false: %v", m.DisableOpenMetrics)
	}

	m = &Metrics{}
	d = kenginefile.NewTestDispenser(`metrics { disable_openmetrics }`)
	err = m.UnmarshalKenginefile(d)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !m.DisableOpenMetrics {
		t.Errorf("DisableOpenMetrics should've been true: %v", m.DisableOpenMetrics)
	}

	m = &Metrics{}
	d = kenginefile.NewTestDispenser(`metrics { bogus }`)
	err = m.UnmarshalKenginefile(d)
	if err == nil {
		t.Errorf("expected error: %v", err)
	}
}
