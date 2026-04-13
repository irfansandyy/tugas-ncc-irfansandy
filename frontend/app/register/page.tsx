"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { FormEvent, useState } from "react";
import { APIError, apiFetch } from "@/lib/api";

export default function RegisterPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setLoading(true);

    try {
      await apiFetch<{ id: number; email: string; username: string }>("/api/auth/register", {
        method: "POST",
        body: JSON.stringify({ email, username, password })
      });
      router.replace("/login");
    } catch (err) {
      const apiError = err as APIError;
      setError(apiError.message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="auth-shell">
      <section className="auth-card">
        <h1>Create Account</h1>
        <p className="auth-subtitle">Register and start chatting with local Llama inference.</p>
        <form className="form-grid" onSubmit={onSubmit}>
          <input
            className="input"
            type="email"
            required
            placeholder="Email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
          <input
            className="input"
            type="text"
            minLength={2}
            maxLength={32}
            placeholder="Username (optional)"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
          />
          <input
            className="input"
            type="password"
            required
            minLength={8}
            placeholder="Password (min 8 chars)"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
          {error ? <p className="error-text">{error}</p> : null}
          <button className="button" disabled={loading} type="submit">
            {loading ? "Creating..." : "Register"}
          </button>
        </form>
        <p className="auth-footer">
          Already have an account? <Link href="/login">Login</Link>
        </p>
      </section>
    </main>
  );
}
