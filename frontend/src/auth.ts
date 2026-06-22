const TOKEN_KEY = "dentvision_token";

export function getStoredToken(): string {
  return localStorage.getItem(TOKEN_KEY) || "";
}

export function storeToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearStoredToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}
