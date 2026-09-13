import { describe, it, expect } from 'vitest'
import { chargesStock } from './digital'

describe('chargesStock', () => {
  it('should say a download cannot run out', () => {
    expect(chargesStock({ digital: { type: 'file' } })).toBe(false)
  })

  it('should say a licence key is stock the shop keeps', () => {
    expect(chargesStock({ digital: { type: 'data' } })).toBe(true)
  })

  it('should say an api product is stock the shop keeps', () => {
    expect(chargesStock({ digital: { type: 'api' } })).toBe(true)
  })

  it('should read goods the shop ships as stock', () => {
    expect(chargesStock({ digital: { type: '' } })).toBe(true)
  })

  it('should read a payload without a delivery type as stock', () => {
    expect(chargesStock({})).toBe(true)
  })
})
