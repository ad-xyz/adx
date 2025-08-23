// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blocklace

import (
	"errors"
	"sync"
	"time"

	"github.com/luxfi/adx/core"
	"github.com/luxfi/adx/pkg/ids"
	"github.com/luxfi/adx/pkg/log"
)

var (
	ErrEquivocation      = errors.New("equivocation detected")
	ErrInvalidPredecessor = errors.New("invalid predecessor reference")
	ErrOrphanBlock       = errors.New("orphan block")
	ErrByzantine         = errors.New("byzantine behavior detected")
)

// DAG represents the Blocklace Directed Acyclic Graph
type DAG struct {
	mu sync.RWMutex
	
	// Graph structure
	vertices  map[ids.ID]*Vertex
	tips      map[ids.ID]*Vertex // Current frontier
	genesis   *Vertex
	
	// Byzantine detection
	equivocations map[ids.NodeID][]*Vertex
	byzantine     map[ids.NodeID]bool
	
	// Ordering
	sequence  []*Vertex // Total order
	delivered map[ids.ID]bool
	
	// Metrics
	height    uint64
	width     int
	
	log       log.Logger
}

// Vertex represents a node in the DAG
type Vertex struct {
	Header       *core.BaseHeader
	Predecessors []*Vertex
	Successors   []*Vertex
	
	// Byzantine tracking
	Author       ids.NodeID
	Round        uint64
	Delivered    bool
	
	// Payload reference
	PayloadHash  []byte
	PayloadPtr   []byte // DA layer pointer
}

// NewDAG creates a new Blocklace DAG
func NewDAG(logger log.Logger) *DAG {
	dag := &DAG{
		vertices:      make(map[ids.ID]*Vertex),
		tips:          make(map[ids.ID]*Vertex),
		equivocations: make(map[ids.NodeID][]*Vertex),
		byzantine:     make(map[ids.NodeID]bool),
		sequence:      make([]*Vertex, 0),
		delivered:     make(map[ids.ID]bool),
		log:           logger,
	}
	
	// Create genesis vertex
	dag.genesis = &Vertex{
		Header: &core.BaseHeader{
			Type:      "genesis",
			ID:        ids.Empty,
			Timestamp: time.Now(),
			Height:    0,
		},
		Author:    ids.EmptyNodeID,
		Round:     0,
		Delivered: true,
	}
	
	dag.vertices[ids.Empty] = dag.genesis
	dag.sequence = append(dag.sequence, dag.genesis)
	
	return dag
}

// AddHeader adds a new header to the DAG
func (d *DAG) AddHeader(header *core.BaseHeader, author ids.NodeID) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	// Check for duplicate
	if _, exists := d.vertices[header.ID]; exists {
		return nil // Already have it
	}
	
	// Verify predecessors
	predecessors := make([]*Vertex, 0, len(header.Predecessors))
	for _, predID := range header.Predecessors {
		pred, exists := d.vertices[predID]
		if !exists {
			return ErrInvalidPredecessor
		}
		predecessors = append(predecessors, pred)
	}
	
	// Create vertex
	vertex := &Vertex{
		Header:       header,
		Predecessors: predecessors,
		Successors:   make([]*Vertex, 0),
		Author:       author,
		Round:        d.calculateRound(predecessors),
	}
	
	// Check for equivocation
	if d.detectEquivocation(vertex) {
		d.handleEquivocation(vertex)
		return ErrEquivocation
	}
	
	// Add to graph
	d.vertices[header.ID] = vertex
	
	// Update predecessors' successors
	for _, pred := range predecessors {
		pred.Successors = append(pred.Successors, vertex)
	}
	
	// Update tips
	d.updateTips(vertex)
	
	// Try to deliver
	d.tryDeliver(vertex)
	
	d.log.Debug("header added to DAG",
		"id", header.ID,
		"type", header.Type,
		"author", author,
		"round", vertex.Round,
		"predecessors", len(predecessors))
	
	return nil
}

// detectEquivocation checks if a vertex represents equivocation
func (d *DAG) detectEquivocation(v *Vertex) bool {
	// Check if author already has a vertex at this round
	authorVertices := d.equivocations[v.Author]
	
	for _, existing := range authorVertices {
		if existing.Round == v.Round {
			// Same author, same round, different vertex = equivocation
			if existing.Header.ID != v.Header.ID {
				return true
			}
		}
	}
	
	// Track this vertex
	d.equivocations[v.Author] = append(authorVertices, v)
	
	return false
}

// handleEquivocation handles detected equivocation
func (d *DAG) handleEquivocation(v *Vertex) {
	// Mark author as Byzantine
	d.byzantine[v.Author] = true
	
	d.log.Warn("equivocation detected",
		"author", v.Author,
		"round", v.Round,
		"vertex", v.Header.ID)
	
	// In Blocklace, equivocating vertices are included but marked
	// This allows the protocol to continue making progress
}

// calculateRound determines the round number for a vertex
func (d *DAG) calculateRound(predecessors []*Vertex) uint64 {
	if len(predecessors) == 0 {
		return 0
	}
	
	maxRound := uint64(0)
	for _, pred := range predecessors {
		if pred.Round > maxRound {
			maxRound = pred.Round
		}
	}
	
	return maxRound + 1
}

// updateTips updates the frontier of undelivered vertices
func (d *DAG) updateTips(v *Vertex) {
	// Add new vertex as tip
	d.tips[v.Header.ID] = v
	
	// Remove predecessors from tips
	for _, pred := range v.Predecessors {
		delete(d.tips, pred.Header.ID)
	}
	
	d.width = len(d.tips)
}

// tryDeliver attempts to deliver vertices in causal order
func (d *DAG) tryDeliver(v *Vertex) {
	// Check if all predecessors are delivered
	for _, pred := range v.Predecessors {
		if !pred.Delivered {
			return // Can't deliver yet
		}
	}
	
	// Deliver this vertex
	v.Delivered = true
	d.delivered[v.Header.ID] = true
	d.sequence = append(d.sequence, v)
	
	if v.Round > d.height {
		d.height = v.Round
	}
	
	d.log.Debug("vertex delivered",
		"id", v.Header.ID,
		"round", v.Round,
		"sequence", len(d.sequence))
	
	// Try to deliver successors
	for _, succ := range v.Successors {
		if !succ.Delivered {
			d.tryDeliver(succ)
		}
	}
}

// GetSequence returns the total order of delivered vertices
func (d *DAG) GetSequence() []*Vertex {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	result := make([]*Vertex, len(d.sequence))
	copy(result, d.sequence)
	return result
}

// GetTips returns the current frontier
func (d *DAG) GetTips() []*Vertex {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	tips := make([]*Vertex, 0, len(d.tips))
	for _, tip := range d.tips {
		tips = append(tips, tip)
	}
	return tips
}

// IsByzantine checks if a node is marked as Byzantine
func (d *DAG) IsByzantine(nodeID ids.NodeID) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	return d.byzantine[nodeID]
}

// GetMetrics returns DAG metrics
func (d *DAG) GetMetrics() DAGMetrics {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	return DAGMetrics{
		Vertices:      len(d.vertices),
		Tips:          len(d.tips),
		Height:        d.height,
		Width:         d.width,
		Delivered:     len(d.sequence),
		Byzantine:     len(d.byzantine),
		Equivocations: len(d.equivocations),
	}
}

// DAGMetrics represents DAG statistics
type DAGMetrics struct {
	Vertices      int
	Tips          int
	Height        uint64
	Width         int
	Delivered     int
	Byzantine     int
	Equivocations int
}

// CordialMiner represents a participant in the Blocklace protocol
type CordialMiner struct {
	ID       ids.NodeID
	DAG      *DAG
	
	// Local state
	round    uint64
	pending  []*core.BaseHeader
	
	// Network interface
	broadcast func(*core.BaseHeader)
	
	log      log.Logger
}

// NewCordialMiner creates a new cordial miner
func NewCordialMiner(id ids.NodeID, dag *DAG, logger log.Logger) *CordialMiner {
	return &CordialMiner{
		ID:      id,
		DAG:     dag,
		pending: make([]*core.BaseHeader, 0),
		log:     logger,
	}
}

// ProposeHeader creates a new header referencing current tips
func (m *CordialMiner) ProposeHeader(headerType core.HeaderType, data []byte) (*core.BaseHeader, error) {
	tips := m.DAG.GetTips()
	
	// Reference all tips as predecessors
	predecessors := make([]ids.ID, 0, len(tips))
	for _, tip := range tips {
		predecessors = append(predecessors, tip.Header.ID)
	}
	
	// Create header
	header := &core.BaseHeader{
		Type:         headerType,
		ID:           ids.GenerateTestID(),
		Timestamp:    time.Now(),
		Predecessors: predecessors,
		Height:       m.round,
		Signature:    data, // Simplified
	}
	
	// Add to local DAG
	if err := m.DAG.AddHeader(header, m.ID); err != nil {
		return nil, err
	}
	
	// Broadcast to network
	if m.broadcast != nil {
		m.broadcast(header)
	}
	
	m.round++
	
	m.log.Debug("header proposed",
		"type", headerType,
		"id", header.ID,
		"round", m.round,
		"predecessors", len(predecessors))
	
	return header, nil
}

// ReceiveHeader processes a header from the network
func (m *CordialMiner) ReceiveHeader(header *core.BaseHeader, sender ids.NodeID) error {
	// Check if sender is Byzantine
	if m.DAG.IsByzantine(sender) {
		m.log.Debug("ignoring header from Byzantine node", "sender", sender)
		return ErrByzantine
	}
	
	// Add to DAG
	if err := m.DAG.AddHeader(header, sender); err != nil {
		if err == ErrEquivocation {
			// Sender equivocated, now marked as Byzantine
			m.log.Warn("equivocation from sender", "sender", sender)
		}
		return err
	}
	
	// Update local round
	sequence := m.DAG.GetSequence()
	if len(sequence) > 0 {
		lastDelivered := sequence[len(sequence)-1]
		if lastDelivered.Round > m.round {
			m.round = lastDelivered.Round
		}
	}
	
	return nil
}