package block

func (m *manager) pushFree(id uint32) {
	if int(id) >= len(m.blocks) {
		return
	}
	b := &m.blocks[id]
	if b.InFreeQueue {
		return
	}

	b.RefCount = 0
	b.TokenCount = 0
	b.InFreeQueue = true
	b.PrevFree = m.freeTail
	b.NextFree = -1

	// Append the block to the free queue tail. If the queue is empty, the block
	// becomes both head and tail.
	if m.freeTail >= 0 {
		m.blocks[m.freeTail].NextFree = int32(id)
	} else {
		m.freeHead = int32(id)
	}
	m.freeTail = int32(id)
	m.freeCount++
}

func (m *manager) popFree() (uint32, bool) {
	if m.freeHead < 0 || m.freeCount == 0 {
		return 0, false
	}

	// Allocate from the free queue head.
	id := uint32(m.freeHead)
	b := &m.blocks[id]
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
