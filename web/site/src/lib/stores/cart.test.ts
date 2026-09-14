import { describe, it, expect, beforeEach } from 'vitest'
import { get } from 'svelte/store'
import { cartStore } from './cart'
import type { CartItem } from '$lib/types/models'

describe('Cart Store', () => {
  beforeEach(() => {
    // Clear cart and localStorage before each test
    cartStore.clear()
    localStorage.clear()
  })

  describe('add', () => {
    it('should add item to empty cart', () => {
      const item: CartItem = {
        id: '1',
        name: 'Test Product',
        slug: 'test-product',
        amount: 1000,
        quantity: 1,
        image: null
      }

      cartStore.add(item)

      const cart = get(cartStore)
      expect(cart).toHaveLength(1)
      expect(cart[0]).toEqual(item)
    })

    it('should not add duplicate item', () => {
      const item: CartItem = {
        id: '1',
        name: 'Test Product',
        slug: 'test-product',
        amount: 1000,
        quantity: 1,
        image: null
      }

      cartStore.add(item)
      cartStore.add(item)

      const cart = get(cartStore)
      expect(cart).toHaveLength(1)
    })

    it('should add multiple different items', () => {
      const item1: CartItem = {
        id: '1',
        name: 'Product 1',
        slug: 'product-1',
        amount: 1000,
        quantity: 1,
        image: null
      }

      const item2: CartItem = {
        id: '2',
        name: 'Product 2',
        slug: 'product-2',
        amount: 2000,
        quantity: 1,
        image: null
      }

      cartStore.add(item1)
      cartStore.add(item2)

      const cart = get(cartStore)
      expect(cart).toHaveLength(2)
      expect(cart[0]).toEqual(item1)
      expect(cart[1]).toEqual(item2)
    })
  })

  describe('remove', () => {
    it('should remove item from cart', () => {
      const item: CartItem = {
        id: '1',
        name: 'Test Product',
        slug: 'test-product',
        amount: 1000,
        quantity: 1,
        image: null
      }

      cartStore.add(item)
      cartStore.remove('1')

      const cart = get(cartStore)
      expect(cart).toHaveLength(0)
    })

    it('should remove only specified item', () => {
      const item1: CartItem = {
        id: '1',
        name: 'Product 1',
        slug: 'product-1',
        amount: 1000,
        quantity: 1,
        image: null
      }

      const item2: CartItem = {
        id: '2',
        name: 'Product 2',
        slug: 'product-2',
        amount: 2000,
        quantity: 1,
        image: null
      }

      cartStore.add(item1)
      cartStore.add(item2)
      cartStore.remove('1')

      const cart = get(cartStore)
      expect(cart).toHaveLength(1)
      expect(cart[0]).toEqual(item2)
    })

    it('should handle removing non-existent item', () => {
      const item: CartItem = {
        id: '1',
        name: 'Product 1',
        slug: 'product-1',
        amount: 1000,
        quantity: 1,
        image: null
      }

      cartStore.add(item)
      cartStore.remove('999')

      const cart = get(cartStore)
      expect(cart).toHaveLength(1)
      expect(cart[0]).toEqual(item)
    })
  })

  describe('clear', () => {
    it('should clear all items from cart', () => {
      const item1: CartItem = {
        id: '1',
        name: 'Product 1',
        slug: 'product-1',
        amount: 1000,
        quantity: 1,
        image: null
      }

      const item2: CartItem = {
        id: '2',
        name: 'Product 2',
        slug: 'product-2',
        amount: 2000,
        quantity: 1,
        image: null
      }

      cartStore.add(item1)
      cartStore.add(item2)
      cartStore.clear()

      const cart = get(cartStore)
      expect(cart).toHaveLength(0)
    })

    it('should clear localStorage', () => {
      const item: CartItem = {
        id: '1',
        name: 'Product 1',
        slug: 'product-1',
        amount: 1000,
        quantity: 1,
        image: null
      }

      cartStore.add(item)
      expect(localStorage.getItem('cart')).not.toBeNull()

      cartStore.clear()
      expect(localStorage.getItem('cart')).toBe('[]')
    })
  })

  describe('persistence', () => {
    it('should persist items to localStorage', () => {
      const item: CartItem = {
        id: '1',
        name: 'Test Product',
        slug: 'test-product',
        amount: 1000,
        quantity: 1,
        image: null
      }

      cartStore.add(item)

      const stored = localStorage.getItem('cart')
      expect(stored).not.toBeNull()
      const parsed = JSON.parse(stored!)
      expect(parsed).toHaveLength(1)
      expect(parsed[0]).toEqual(item)
    })
  })

  describe('one copy of a download', () => {
    const download: CartItem = {
      id: 'guide',
      name: 'A Guide',
      slug: 'a-guide',
      amount: 2400,
      quantity: 1,
      image: null,
      digital: { type: 'file' }
    }

    it('should hold one copy however many were asked for', () => {
      cartStore.add({ ...download, quantity: 3 })

      expect(get(cartStore)[0].quantity).toBe(1)
    })

    it('should not accumulate a second copy', () => {
      cartStore.add(download)
      cartStore.add(download)

      expect(get(cartStore)[0].quantity).toBe(1)
    })

    it('should not step past one copy', () => {
      cartStore.add(download)
      cartStore.incrementQuantity(download.id, undefined)

      expect(get(cartStore)[0].quantity).toBe(1)
    })

    it('should not be set to more than one copy', () => {
      cartStore.add(download)
      cartStore.updateQuantity(download.id, undefined, 5)

      expect(get(cartStore)[0].quantity).toBe(1)
    })

    it('should leave a licence key with the number it was given', () => {
      cartStore.add({ ...download, id: 'keys', digital: { type: 'data' }, quantity: 3 })
      cartStore.incrementQuantity('keys', undefined)

      expect(get(cartStore)[0].quantity).toBe(4)
    })

    // A cart written by a version that did not yet sell a download as one copy
    // is read back through the same rule: the list and the total under it are
    // the buyer's price, and the checkout would charge one.
    it('should bring a stored download back as one copy', () => {
      localStorage.setItem('cart', JSON.stringify([{ ...download, quantity: 3 }]))

      cartStore.reload()

      expect(get(cartStore)[0].quantity).toBe(1)
    })
  })
})
