package doctor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDSN_Empty(t *testing.T) {
	c := checkDSN("")
	assert.Equal(t, StatusFail, c.Status)
	assert.Equal(t, "dsn_valid", c.Name)
}

func TestCheckDSN_Valid(t *testing.T) {
	c := checkDSN("postgres://testuser@localhost:5432/testdb")
	assert.Equal(t, StatusPass, c.Status)
}

func TestCheckDSN_KeyValue(t *testing.T) {
	c := checkDSN("host=localhost port=5432 dbname=db user=app")
	assert.Equal(t, StatusPass, c.Status)
}

func TestWorstStatus(t *testing.T) {
	tests := []struct {
		name   string
		checks []Check
		want   CheckStatus
	}{
		{
			name:   "all pass",
			checks: []Check{{Status: StatusPass}, {Status: StatusPass}},
			want:   StatusPass,
		},
		{
			name:   "one warn",
			checks: []Check{{Status: StatusPass}, {Status: StatusWarn}},
			want:   StatusWarn,
		},
		{
			name:   "one fail",
			checks: []Check{{Status: StatusPass}, {Status: StatusWarn}, {Status: StatusFail}},
			want:   StatusFail,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, worstStatus(tt.checks))
		})
	}
}

func TestRunAll_BadDSN(t *testing.T) {
	r := RunAll(context.Background(), "", "test")
	assert.Equal(t, StatusFail, r.Status)
	assert.Len(t, r.Checks, 1)
	assert.Equal(t, "dsn_valid", r.Checks[0].Name)
}

func TestRunAll_BadConnectivity(t *testing.T) {
	r := RunAll(context.Background(), "postgres://nohost:5432/nodb?connect_timeout=1", "test")
	assert.Equal(t, StatusFail, r.Status)
	assert.Len(t, r.Checks, 2)
	assert.Equal(t, "connectivity", r.Checks[1].Name)
	assert.Equal(t, StatusFail, r.Checks[1].Status)
}
