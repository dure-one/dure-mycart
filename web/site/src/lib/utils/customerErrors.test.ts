import { describe, it, expect } from 'vitest'
import { customerErrorKey } from './customerErrors'

describe('customerErrorKey', () => {
  it('names the sentence a wrong sign-in is answered with', () => {
    expect(customerErrorKey('wrong email or password')).toBe('account.wrongCredentials')
  })

  it('names the sentence a taken address is answered with', () => {
    expect(customerErrorKey('this email is already registered')).toBe('account.emailTaken')
  })

  it('reads every other answer as the generic failure', () => {
    expect(customerErrorKey('Internal Server Error')).toBe('account.requestFailed')
    expect(customerErrorKey(undefined)).toBe('account.requestFailed')
  })
})
