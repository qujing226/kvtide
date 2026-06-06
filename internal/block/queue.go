package block

import "github.com/qujing226/mini-llm-serve/internal/model"

func (m *manager) pushFree(id uint32) {
	b := &m.blocks[id]
	if b.RefCount == 0 {
		return
	}
	b.RefCount--

	if b.RefCount > 0 {
		return
	}
	m.appendFreeBlock(b)
}

func (m *manager) popFree() (uint32, bool) {
	if m.freeHead < 0 || m.freeCount == 0 {
		return 0, false
	}

	// Allocate from the free queue head.
	id := uint32(m.freeHead)
	b := &m.blocks[id]
	if b.Cached {
		delete(m.cachedBlocks, b.Hash)
		b.Cached = false
	}
	b.Hash = ""
	b.TokenCount = 0
	b.RefCount = 1

	next := b.NextFree

	m.freeHead = next
	if next >= 0 {
		// The next block becomes the new head.
		m.blocks[next].PrevFree = -1
	} else {
		// The popped block was the last free block.
		m.freeTail = -1
	}

	b.InFreeQueue = false
	b.PrevFree = -1
	b.NextFree = -1
	m.freeCount--

	return id, true
}

func (m *manager) appendFreeBlock(b *model.Block) {
	b.RefCount = 0
	b.InFreeQueue = true
	b.PrevFree = m.freeTail
	b.NextFree = -1

	if m.freeTail >= 0 {
		m.blocks[m.freeTail].NextFree = int32(b.ID)
	} else {
		m.freeHead = int32(b.ID)
	}

	m.freeTail = int32(b.ID)
	m.freeCount++
}
