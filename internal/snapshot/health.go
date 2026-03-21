package snapshot

// DeriveStatus computes the overall severity from collected data.
func DeriveStatus(s *Snapshot) Severity {
	if isCritical(s) {
		return SeverityCritical
	}
	if isDegraded(s) {
		return SeverityDegraded
	}
	return SeverityHealthy
}

func isCritical(s *Snapshot) bool {
	if s.Connections.Max > 0 && s.Connections.UsedRatio > 0.9 {
		return true
	}
	if s.Locks.ChainMaxDepth > 5 {
		return true
	}
	for _, r := range s.Replication {
		if r.LagSeconds > 300 {
			return true
		}
	}
	return false
}

func isDegraded(s *Snapshot) bool {
	if s.Connections.Max > 0 && s.Connections.UsedRatio > 0.75 {
		return true
	}
	if s.Locks.ChainMaxDepth > 2 {
		return true
	}
	if s.Connections.SlowCount > 0 {
		return true
	}
	for _, v := range s.Vacuum.Tables {
		if v.DeadTupleRatio > 0.2 {
			return true
		}
	}
	for _, r := range s.Replication {
		if r.LagSeconds > 30 {
			return true
		}
	}
	return false
}

// FilterUnhealthy removes healthy items from per-table slices and returns
// a filtered copy. Returns nil if everything is healthy.
func FilterUnhealthy(s *Snapshot) *Snapshot {
	out := *s

	var vacTables []VacuumTableData
	for _, v := range s.Vacuum.Tables {
		if v.DeadTupleRatio > 0.1 {
			vacTables = append(vacTables, v)
		}
	}
	out.Vacuum.Tables = vacTables

	var bloat []BloatEntry
	for _, b := range s.Bloat {
		if b.BloatRatio > 0.1 {
			bloat = append(bloat, b)
		}
	}
	out.Bloat = bloat

	var stats []TableStatData
	for _, t := range s.TableStats {
		if t.SeqScanRatio > 0.5 && t.SeqScans > 1000 {
			stats = append(stats, t)
		}
	}
	out.TableStats = stats

	var replicas []ReplicaData
	for _, r := range s.Replication {
		if r.LagSeconds > 5 {
			replicas = append(replicas, r)
		}
	}
	out.Replication = replicas

	return &out
}
