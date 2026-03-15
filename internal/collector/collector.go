package collector

import (
	"context"
	"log"
	"time"

	"github.com/ppiankov/pgpulse/internal/config"
	"github.com/ppiankov/pgpulse/internal/metrics"
)

type Collector struct {
	db                  Querier
	metrics             *metrics.Metrics
	cfg                 config.Config
	hasStmt             bool
	useV13              bool
	hasPG14             bool
	hasPG17             bool
	prevStmts           map[string]stmtSnapshot
	regressionThreshold float64
	prevWalBytes        float64
	prevWalTime         time.Time
	connHistory         []connSample
}

func New(db Querier, m *metrics.Metrics, cfg config.Config) *Collector {
	threshold := cfg.RegressionThreshold
	if threshold <= 0 {
		threshold = 2.0
	}
	return &Collector{
		db:                  db,
		metrics:             m,
		cfg:                 cfg,
		regressionThreshold: threshold,
	}
}

// ProbeExtensions checks if pg_stat_statements is available and detects the PG version.
func (c *Collector) ProbeExtensions(ctx context.Context) {
	row := c.db.QueryRowContext(ctx, "SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements'")
	var one int
	if err := row.Scan(&one); err != nil {
		log.Println("pg_stat_statements not available, statement metrics will be skipped")
		c.hasStmt = false
		return
	}
	c.hasStmt = true

	// Detect PG version for correct column names
	row = c.db.QueryRowContext(ctx, "SHOW server_version_num")
	var versionNum int
	if err := row.Scan(&versionNum); err != nil {
		log.Printf("could not detect PG version, assuming v13+: %v", err)
		c.useV13 = true
		return
	}
	c.useV13 = versionNum >= 130000
	c.hasPG14 = versionNum >= 140000
	c.hasPG17 = versionNum >= 170000
	log.Printf("PostgreSQL version %d detected, pg_stat_statements v13 columns: %v", versionNum, c.useV13)
}

func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.PollInterval)
	defer ticker.Stop()

	// Collect immediately on start
	c.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

func (c *Collector) collect(ctx context.Context) {
	start := time.Now()

	totalConns, err := collectActivity(ctx, c.db, c.metrics, c.cfg.SlowQueryThreshold.Seconds())
	if err != nil {
		log.Printf("activity collection error: %v", err)
		c.metrics.ScrapeErrors.Inc()
		c.metrics.Up.Set(0)
		c.metrics.ScrapeDuration.Set(time.Since(start).Seconds())
		return
	}

	if err := collectDatabase(ctx, c.db, c.metrics, totalConns); err != nil {
		log.Printf("database collection error: %v", err)
		c.metrics.ScrapeErrors.Inc()
	}

	if c.hasStmt {
		if err := collectStatements(ctx, c.db, c.metrics, c.useV13); err != nil {
			log.Printf("statements collection error: %v", err)
			c.metrics.ScrapeErrors.Inc()
		}
	}

	if err := collectVacuum(ctx, c.db, c.metrics); err != nil {
		log.Printf("vacuum collection error: %v", err)
		c.metrics.ScrapeErrors.Inc()
	}

	if err := collectBloat(ctx, c.db, c.metrics); err != nil {
		log.Printf("bloat collection error: %v", err)
		c.metrics.ScrapeErrors.Inc()
	}

	if err := collectLocks(ctx, c.db, c.metrics); err != nil {
		log.Printf("locks collection error: %v", err)
		c.metrics.ScrapeErrors.Inc()
	}

	if c.hasStmt {
		if err := c.collectRegression(ctx); err != nil {
			log.Printf("regression collection error: %v", err)
			c.metrics.ScrapeErrors.Inc()
		}
	}

	if err := collectReplication(ctx, c.db, c.metrics); err != nil {
		log.Printf("replication collection error: %v", err)
		c.metrics.ScrapeErrors.Inc()
	}

	if err := collectConnLifecycle(ctx, c.db, c.metrics); err != nil {
		log.Printf("connection lifecycle collection error: %v", err)
		c.metrics.ScrapeErrors.Inc()
	}

	if err := collectTableStats(ctx, c.db, c.metrics); err != nil {
		log.Printf("table stats collection error: %v", err)
		c.metrics.ScrapeErrors.Inc()
	}

	if c.hasPG14 {
		if err := c.collectWAL(ctx); err != nil {
			log.Printf("WAL collection error: %v", err)
			c.metrics.ScrapeErrors.Inc()
		}
	}

	if err := c.collectCheckpoint(ctx); err != nil {
		log.Printf("checkpoint collection error: %v", err)
		c.metrics.ScrapeErrors.Inc()
	}

	c.collectPrediction(ctx, totalConns)

	c.metrics.Up.Set(1)
	c.metrics.ScrapeDuration.Set(time.Since(start).Seconds())
}
