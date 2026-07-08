import { describe, expect, it } from 'vitest';
import { statusColor, statusText } from './status';

describe('status helpers', () => {
  it('maps common status values', () => {
    expect(statusColor('online')).toBe('green');
    expect(statusColor('running')).toBe('blue');
    expect(statusColor('failed')).toBe('red');
    expect(statusText('timeout')).toBe('超时');
  });
});
