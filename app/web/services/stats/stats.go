package stats

import (
	"strconv"

	"github.com/wzshiming/jumpway/metrics"
)

// Source is the running registry the service reads and manages.
type Source interface {
	Snapshot() metrics.Snapshot
	Reset()
	Disconnect(id uint64) error
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

// Disconnect closes one current connection
// #route:"DELETE /connections/{id}"#
func (s *StatsService) Disconnect(id string /* #name:"id"# */) (err error) {
	parsed, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return err
	}
	return s.source.Disconnect(parsed)
}
