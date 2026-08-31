import { describe, expect, it } from 'vitest';
import { receivedIndexes } from './queue';

describe('reliable upload protocol helpers', () => {
  it('expands compact received part ranges', () => {
    const received = receivedIndexes({ id: 'a', path: '', name: 'a.txt', size: 12, chunk_bytes: 4, part_count: 3, received: [{ start: 0, end: 1 }, { start: 3, end: 3 }], received_bytes: 8, state: 'active', expires_at: '2026-01-01T00:00:00Z' });
    expect([...received]).toEqual([0, 1, 3]);
  });
});
