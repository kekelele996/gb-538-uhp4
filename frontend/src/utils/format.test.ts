import { describe, expect, it } from 'vitest'
import { signedFixed } from './format'

describe('signedFixed', () => {
  it('formats positive deltas with an explicit plus sign', () => {
    expect(signedFixed(5.5, 2)).toBe('+5.50')
    expect(signedFixed(0.123, 2, ' pp')).toBe('+0.12 pp')
  })

  it('formats negative deltas with a minus sign and absolute value', () => {
    expect(signedFixed(-6.351, 2, ' pp')).toBe('−6.35 pp')
    expect(signedFixed(-0.00001, 2, ' dB')).toBe('−0.00 dB')
  })

  it('treats zero and negative zero as unsigned', () => {
    expect(signedFixed(0, 1, '%')).toBe('0.0%')
    expect(signedFixed(-0, 2)).toBe('0.00')
  })

  it('guards non-finite values', () => {
    expect(signedFixed(Number.NaN, 2)).toBe('—')
    expect(signedFixed(undefined, 2)).toBe('—')
  })
})
