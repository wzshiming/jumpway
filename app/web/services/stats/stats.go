package stats

import "github.com/wzshiming/jumpway/metrics"

// Source is the running registry the service reads and resets.
type Source interface {
	Snapshot() metrics.Snapshot
	Reset()
}

// StatsService exposes traffic statistics of the running rules
// #path:"/stats/"#
type StatsService struct {
	source Source
}

// NewStatsService Create a new StatsService
func NewStatsService(source Source) *StatsService {
	return &StatsService{source: source}
}

// Get the traffic statistics of every enabled rule
// #route:"GET /"#
func (s *StatsService) Get() (snapshot *metrics.Snapshot, err error) {
	current := s.source.Snapshot()
	return &current, nil
}

// Reset all traffic statistics
// #route:"DELETE /"#
func (s *StatsService) Reset() (err error) {
	s.source.Reset()
	return nil
}
