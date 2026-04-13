"use client";

import Link from "next/link";
import { FormEvent, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { APIError, apiFetch } from "@/lib/api";
import { clearSession, getEmail, getToken, getUsername, setUsername } from "@/lib/auth";

type ProfileResponse = {
  id: number;
  email: string;
  username: string;
};

export default function SettingsPage() {
  const router = useRouter();
  const token = useMemo(() => getToken(), []);
  const [email, setEmail] = useState("");
  const [username, setUsernameInput] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  useEffect(() => {
    if (!token) {
      clearSession();
      router.replace("/login");
      return;
    }

    setEmail(getEmail() ?? "");
    setUsernameInput(getUsername() ?? "");

    async function loadProfile() {
      try {
        const profile = await apiFetch<ProfileResponse>("/api/me", { method: "GET" }, token);
        setEmail(profile.email);
        setUsernameInput(profile.username);
        setUsername(profile.username);
      } catch (err) {
        const apiError = err as APIError;
        if (apiError.status === 401) {
          clearSession();
          router.replace("/login");
          return;
        }
        setError(apiError.message);
      } finally {
        setLoading(false);
      }
    }

    void loadProfile();
  }, [router, token]);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token) {
      return;
    }

    setSaving(true);
    setError(null);
    setSuccess(null);

    try {
      const profile = await apiFetch<ProfileResponse>(
        "/api/me",
        {
          method: "PATCH",
          body: JSON.stringify({ username })
        },
        token
      );
      setUsernameInput(profile.username);
      setUsername(profile.username);
      setSuccess("Username updated successfully.");
    } catch (err) {
      const apiError = err as APIError;
      if (apiError.status === 401) {
        clearSession();
        router.replace("/login");
        return;
      }
      setError(apiError.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <main className="auth-shell">
      <section className="auth-card">
        <h1>Profile Settings</h1>
        <p className="auth-subtitle">Update your username shown in the chat header.</p>
        {loading ? (
          <p className="muted">Loading profile...</p>
        ) : (
          <form className="form-grid" onSubmit={onSubmit}>
            <input className="input" type="email" value={email} disabled />
            <input
              className="input"
              type="text"
              required
              minLength={2}
              maxLength={32}
              placeholder="Username"
              value={username}
              onChange={(event) => setUsernameInput(event.target.value)}
            />
            {error ? <p className="error-text">{error}</p> : null}
            {success ? <p className="muted">{success}</p> : null}
            <button className="button" type="submit" disabled={saving}>
              {saving ? "Saving..." : "Save Username"}
            </button>
          </form>
        )}
        <p className="auth-footer">
          <Link href="/chat">Back to chat</Link>
        </p>
      </section>
    </main>
  );
}
