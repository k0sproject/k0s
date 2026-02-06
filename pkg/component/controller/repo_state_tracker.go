// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"sync"
	"time"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// RepoStatus represents the current state of a repository
type RepoStatus int

const (
	RepoStatusPending RepoStatus = iota
	RepoStatusReady
	RepoStatusRetriableError
	RepoStatusPermanentError
)

// RepoState tracks the initialization state of a Helm repository
type RepoState struct {
	Status      RepoStatus
	Repository  k0sv1beta1.Repository
	LastError   string
	NextRetryAt time.Time
}

// repoStateTracker manages repository states in a thread-safe manner
type repoStateTracker struct {
	states sync.Map
}

// newRepoStateTracker creates a new repository state tracker
func newRepoStateTracker() *repoStateTracker {
	return &repoStateTracker{}
}

// MarkPending marks a repository as pending initialization
func (t *repoStateTracker) MarkPending(repo k0sv1beta1.Repository) {
	t.states.Store(repo.Name, &RepoState{
		Status:     RepoStatusPending,
		Repository: repo,
	})
}

// MarkReady marks a repository as successfully initialized
func (t *repoStateTracker) MarkReady(repo k0sv1beta1.Repository) {
	t.states.Store(repo.Name, &RepoState{
		Status:     RepoStatusReady,
		Repository: repo,
	})
}

// MarkRetriableError marks a repository as having a retriable error
func (t *repoStateTracker) MarkRetriableError(repo k0sv1beta1.Repository, err error, nextRetry time.Duration) {
	t.states.Store(repo.Name, &RepoState{
		Status:      RepoStatusRetriableError,
		Repository:  repo,
		LastError:   err.Error(),
		NextRetryAt: time.Now().Add(nextRetry),
	})
}

// MarkPermanentError marks a repository as having a permanent error
func (t *repoStateTracker) MarkPermanentError(repo k0sv1beta1.Repository, err error) {
	t.states.Store(repo.Name, &RepoState{
		Status:     RepoStatusPermanentError,
		Repository: repo,
		LastError:  err.Error(),
	})
}

// GetState returns the current state of a repository
func (t *repoStateTracker) GetState(repoName string) *RepoState {
	value, exists := t.states.Load(repoName)
	if !exists {
		return nil
	}

	state := value.(*RepoState)
	// Return a copy to prevent external modifications
	stateCopy := *state
	return &stateCopy
}
