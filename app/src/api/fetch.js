export async function apiFetch(path, options = {}) {
  const { allow404, fallback, ...fetchOptions } = options

  const res = await fetch(path, { credentials: 'include', ...fetchOptions })

  if (res.status === 401) {
    window.dispatchEvent(new Event('auth:expired'))
    throw new Error('Unauthorized')
  }

  if (res.status === 404 && allow404) {
    return fallback
  }

  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}`)
  }

  return res.json()
}
