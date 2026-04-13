"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { FormEvent, useState } from "react";
import { APIError, apiFetch } from "@/lib/api";
import { setSession } from "@/lib/auth";

type LoginResponse = {
  token: string;
  user: {
    id: number;
    email: string;
    username: string;
  };
};

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const response = await apiFetch<LoginResponse>("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password })
      });
      setSession(response.token, response.user.email, response.user.username);
      router.replace("/chat");
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
        <h1>Welcome Back</h1>
        <p className="auth-subtitle">Sign in to access your private Llama chat history.</p>
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
            type="password"
            required
            minLength={8}
            placeholder="Password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
          {error ? <p className="error-text">{error}</p> : null}
          <button className="button" disabled={loading} type="submit">
            {loading ? "Signing in..." : "Login"}
          </button>
        </form>
        <p className="auth-footer">
          Need an account? <Link href="/register">Register here</Link>
        </p>
      </section>
    </main>
  );
}
