package state

import (
	"sync"
	"time"
)

type ContainerRow struct {
	ID          string   `json:"id"`
	Names       []string `json:"names"`
	Image       string   `json:"image"`
	State       string   `json:"state"`
	Project     string   `json:"project"`
	Service     string   `json:"service"`
	Enabled     bool     `json:"enabled"`
	UpToDate    bool     `json:"upToDate"`
	HasError    bool     `json:"hasError"`
	LastChecked string   `json:"lastChecked"`
}

type DeploymentRow struct {
	Time    string `json:"time"`
	Project string `json:"project"`
	Service string `json:"service"`
	Image   string `json:"image"`
	OldRef  string `json:"oldRef"`
	NewRef  string `json:"newRef"`
}

type Store struct {
	mu          sync.RWMutex
	containers  []ContainerRow
	lastChecked time.Time
	deployments []DeploymentRow
}

func New() *Store {
	return &Store{}
}

func (s *Store) Containers() []ContainerRow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ContainerRow, len(s.containers))
	copy(out, s.containers)
	return out
}

func (s *Store) UpdateContainers(rows []ContainerRow, checkedAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.containers = rows
	s.lastChecked = checkedAt
}

func (s *Store) ReplaceContainerID(oldID, newID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.containers {
		if c.ID == oldID {
			s.containers[i].ID = newID
			return
		}
	}
}

func (s *Store) MarkContainerChecked(id string, upToDate bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Format(time.RFC3339)
	for i, c := range s.containers {
		if c.ID == id {
			s.containers[i].UpToDate = upToDate
			s.containers[i].HasError = false
			s.containers[i].LastChecked = now
			return
		}
	}
}

func (s *Store) MarkContainerError(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Format(time.RFC3339)
	for i, c := range s.containers {
		if c.ID == id {
			s.containers[i].HasError = true
			s.containers[i].LastChecked = now
			return
		}
	}
}

func (s *Store) AddDeployment(d DeploymentRow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deployments = append(s.deployments, d)
	if len(s.deployments) > 100 {
		s.deployments = s.deployments[len(s.deployments)-100:]
	}
}

func (s *Store) LastChecked() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lastChecked.IsZero() {
		return ""
	}
	return s.lastChecked.Format(time.RFC3339)
}

func (s *Store) Deployments() []DeploymentRow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DeploymentRow, len(s.deployments))
	copy(out, s.deployments)
	return out
}
