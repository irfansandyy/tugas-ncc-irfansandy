"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { APIError, apiFetch } from "@/lib/api";
import { clearSession, getEmail, getToken } from "@/lib/auth";

type Chat = {
  id: number;
  title: string;
  updated_at: string;
};

type Message = {
  id: number;
  role: "user" | "assistant";
  content: string;
  created_at: string;
};

type ChatShellProps = {
  activeChatId?: number;
};

type ChatsResponse = {
  items: Chat[];
};

type MessagesResponse = {
  items: Message[];
};

type SendMessageResponse = {
  user_message: Message;
  assistant_message: Message;
};

export default function ChatShell({ activeChatId }: ChatShellProps) {
  const router = useRouter();
  const [chats, setChats] = useState<Chat[]>([]);
  const [messages, setMessages] = useState<Message[]>([]);
  const [draft, setDraft] = useState("");
  const [loadingChats, setLoadingChats] = useState(true);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [userEmail, setUserEmail] = useState<string>("");

  const token = useMemo(() => getToken(), []);

  const handleUnauthorized = useCallback(() => {
    clearSession();
    router.replace("/login");
  }, [router]);

  const loadChats = useCallback(async () => {
    if (!token) {
      handleUnauthorized();
      return;
    }

    setLoadingChats(true);
    try {
      const response = await apiFetch<ChatsResponse>("/api/chats", { method: "GET" }, token);
      setChats(response.items);
    } catch (err) {
      const apiError = err as APIError;
      if (apiError.status === 401) {
        handleUnauthorized();
        return;
      }
      setError(apiError.message);
    } finally {
      setLoadingChats(false);
    }
  }, [handleUnauthorized, token]);

  const loadMessages = useCallback(
    async (chatId: number) => {
      if (!token) {
        handleUnauthorized();
        return;
      }

      try {
        const response = await apiFetch<MessagesResponse>(
          `/api/chats/${chatId}/messages`,
          { method: "GET" },
          token
        );
        setMessages(response.items);
      } catch (err) {
        const apiError = err as APIError;
        if (apiError.status === 401) {
          handleUnauthorized();
          return;
        }
        setError(apiError.message);
      }
    },
    [handleUnauthorized, token]
  );

  useEffect(() => {
    if (!token) {
      handleUnauthorized();
      return;
    }
    setUserEmail(getEmail() ?? "");
    void loadChats();
  }, [handleUnauthorized, loadChats, token]);

  useEffect(() => {
    if (!activeChatId) {
      setMessages([]);
      return;
    }
    void loadMessages(activeChatId);
  }, [activeChatId, loadMessages]);

  async function createChat(title: string): Promise<Chat> {
    const response = await apiFetch<Chat>(
      "/api/chats",
      {
        method: "POST",
        body: JSON.stringify({ title })
      },
      token
    );
    setChats((prev) => [response, ...prev]);
    return response;
  }

  async function handleNewChat() {
    if (!token) {
      handleUnauthorized();
      return;
    }
    setError(null);
    try {
      const chat = await createChat("New Chat");
      router.push(`/chat/${chat.id}`);
    } catch (err) {
      const apiError = err as APIError;
      setError(apiError.message);
    }
  }

  async function handleSendMessage(event: FormEvent) {
    event.preventDefault();
    if (!draft.trim() || !token || sending) {
      return;
    }

    const prompt = draft.trim();
    setDraft("");
    setSending(true);
    setError(null);

    try {
      let chatId = activeChatId;
      if (!chatId) {
        const created = await createChat(prompt.slice(0, 42));
        chatId = created.id;
        router.push(`/chat/${chatId}`);
      }

      const response = await apiFetch<SendMessageResponse>(
        `/api/chats/${chatId}/messages`,
        {
          method: "POST",
          body: JSON.stringify({ content: prompt })
        },
        token
      );

      setMessages((prev) => [...prev, response.user_message, response.assistant_message]);
      await loadChats();
    } catch (err) {
      const apiError = err as APIError;
      if (apiError.status === 401) {
        handleUnauthorized();
        return;
      }
      setError(apiError.message);
    } finally {
      setSending(false);
    }
  }

  function logout() {
    clearSession();
    router.replace("/login");
  }

  return (
    <div className="chat-page">
      <aside className="chat-sidebar">
        <h2 className="chat-brand">Llama Chat</h2>
        <button className="button" type="button" onClick={handleNewChat}>
          New Chat
        </button>

        <ul className="chat-list">
          {loadingChats ? <li className="muted">Loading chats...</li> : null}
          {!loadingChats && chats.length === 0 ? <li className="muted">No chat history yet.</li> : null}
          {chats.map((chat) => (
            <li key={chat.id} className={`chat-list-item ${activeChatId === chat.id ? "active" : ""}`}>
              <Link href={`/chat/${chat.id}`}>{chat.title}</Link>
            </li>
          ))}
        </ul>
      </aside>

      <main className="chat-main">
        <header className="chat-header">
          <div>
            <h1 className="chat-title">{activeChatId ? `Chat #${activeChatId}` : "Start a conversation"}</h1>
            <small className="muted">{userEmail || "Authenticated user"}</small>
          </div>
          <button className="button button-ghost chat-logout" type="button" onClick={logout}>
            Logout
          </button>
        </header>

        <section className="chat-messages">
          {!activeChatId ? <p className="muted">Select a chat from the sidebar or create a new one.</p> : null}
          {activeChatId && messages.length === 0 ? <p className="muted">No messages yet. Send the first prompt.</p> : null}
          {messages.map((msg) => (
            <article key={msg.id} className={`message ${msg.role}`}>
              {msg.content}
            </article>
          ))}
        </section>

        <form className="chat-composer" onSubmit={handleSendMessage}>
          {error ? <p className="error-text">{error}</p> : null}
          <div className="composer-row">
            <input
              className="input"
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              placeholder="Type your message..."
              disabled={sending}
            />
            <button className="button" type="submit" disabled={sending}>
              {sending ? "Sending..." : "Send"}
            </button>
          </div>
        </form>
      </main>
    </div>
  );
}
