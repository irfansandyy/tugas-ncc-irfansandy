const TOKEN_KEY = "chat_app_token";
const EMAIL_KEY = "chat_app_email";
const USERNAME_KEY = "chat_app_username";

export function getToken(): string | null {
  if (typeof window === "undefined") {
    return null;
  }
  return localStorage.getItem(TOKEN_KEY);
}

export function setSession(token: string, email: string, username?: string): void {
  localStorage.setItem(TOKEN_KEY, token);
  localStorage.setItem(EMAIL_KEY, email);
  if (typeof username === "string") {
    localStorage.setItem(USERNAME_KEY, username);
  }
}

export function clearSession(): void {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(EMAIL_KEY);
  localStorage.removeItem(USERNAME_KEY);
}

export function getEmail(): string | null {
  if (typeof window === "undefined") {
    return null;
  }
  return localStorage.getItem(EMAIL_KEY);
}

export function getUsername(): string | null {
  if (typeof window === "undefined") {
    return null;
  }
  return localStorage.getItem(USERNAME_KEY);
}

export function setUsername(username: string): void {
  localStorage.setItem(USERNAME_KEY, username);
}
