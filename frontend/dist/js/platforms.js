// Single source of truth for platform metadata, shared by the home grid,
// the app detail page, and the admin upload/release lists so the three
// surfaces never drift out of sync with each other or with the server's
// own detection in handlers/apps.go#detectPlatform.
export const PLATFORMS = {
  android: { label: 'Android', code: 'AND', dot: '#3ddc84' },
  ios: { label: 'iOS', code: 'IOS', dot: '#3a8dff' },
  windows: { label: 'Windows', code: 'WIN', dot: '#23c4f0' },
  mac: { label: 'Mac', code: 'MAC', dot: '#b9bbc6' },
  linux: { label: 'Linux', code: 'LNX', dot: '#f5b400' },
  other: { label: 'Other', code: 'PKG', dot: '#8786a6' },
}

const EXTENSION_PLATFORMS = {
  apk: 'android', aab: 'android',
  ipa: 'ios',
  exe: 'windows', msi: 'windows',
  dmg: 'mac', pkg: 'mac',
  deb: 'linux', rpm: 'linux', appimage: 'linux',
}

export function platformInfo(key) {
  return PLATFORMS[(key || '').toLowerCase()] || PLATFORMS.other
}

export function detectPlatformFromFilename(filename) {
  const ext = filename.toLowerCase().split('.').pop()
  return EXTENSION_PLATFORMS[ext] || 'other'
}
