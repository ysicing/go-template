// This hook is no longer needed as token refresh is handled automatically
// by axios interceptors in api/client.ts using HttpOnly cookies
export function useTokenRefresh() {
  // No-op: tokens are managed by HttpOnly cookies
}
