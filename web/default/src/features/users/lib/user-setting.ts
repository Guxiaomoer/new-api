import type { User, UserSetting } from '../types'

export function parseUserSetting(setting?: string): UserSetting {
  if (!setting) return {}
  try {
    const parsed = JSON.parse(setting)
    return typeof parsed === 'object' && parsed !== null ? parsed : {}
  } catch {
    return {}
  }
}

export function isUserApiRestricted(user: User): boolean {
  return parseUserSetting(user.setting).api_restricted === true
}

export function getUserApiRestrictedMessage(user: User): string {
  return parseUserSetting(user.setting).api_restricted_message || ''
}
