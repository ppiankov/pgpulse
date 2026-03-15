package collector

import (
	"context"

	"github.com/ppiankov/pgpulse/internal/metrics"
)

const lockChainsQuery = `
SELECT
    blocked.pid AS blocked_pid,
    blocking.pid AS blocking_pid,
    blocked.locktype
FROM pg_locks blocked
JOIN pg_locks blocking
    ON blocking.locktype = blocked.locktype
    AND blocking.relation IS NOT DISTINCT FROM blocked.relation
    AND blocking.pid != blocked.pid
    AND blocking.granted
WHERE NOT blocked.granted
`

func collectLocks(ctx context.Context, db Querier, m *metrics.Metrics) error {
	rows, err := db.QueryContext(ctx, lockChainsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	typeCounts := make(map[string]float64)
	adj := make(map[int][]int)    // blocker -> []blocked
	blocked := make(map[int]bool) // PIDs that are blocked
	var totalBlocked float64

	for rows.Next() {
		var blockedPID, blockingPID int
		var lockType string

		if err := rows.Scan(&blockedPID, &blockingPID, &lockType); err != nil {
			return err
		}

		totalBlocked++
		typeCounts[lockType]++
		adj[blockingPID] = append(adj[blockingPID], blockedPID)
		blocked[blockedPID] = true
	}

	if err := rows.Err(); err != nil {
		return err
	}

	m.LockBlockedQueries.Set(totalBlocked)

	m.LockByType.Reset()
	for lt, count := range typeCounts {
		m.LockByType.WithLabelValues(lt).Set(count)
	}

	m.LockChainMaxDepth.Set(float64(maxChainDepth(adj, blocked)))

	return nil
}

// maxChainDepth computes the longest lock wait chain using iterative DFS.
// Capped at 10 to prevent runaway computation on cycles.
func maxChainDepth(adj map[int][]int, blocked map[int]bool) int {
	maxDepth := 0

	for root := range adj {
		if blocked[root] {
			continue // not a root — this PID is itself blocked
		}

		// Iterative DFS from this root.
		type frame struct {
			pid   int
			depth int
		}
		stack := []frame{{root, 0}}
		visited := make(map[int]bool)

		for len(stack) > 0 {
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			if visited[f.pid] || f.depth > 10 {
				continue
			}
			visited[f.pid] = true

			if f.depth > maxDepth {
				maxDepth = f.depth
			}

			for _, child := range adj[f.pid] {
				stack = append(stack, frame{child, f.depth + 1})
			}
		}
	}

	return maxDepth
}
