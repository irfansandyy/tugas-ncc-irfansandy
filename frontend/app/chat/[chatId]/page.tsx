import ChatShell from "@/components/chat-shell";

type ChatPageProps = {
  params:
    | {
        chatId: string;
      }
    | Promise<{
        chatId: string;
      }>;
};

export default async function ChatPage({ params }: ChatPageProps) {
  const resolvedParams = await Promise.resolve(params);
  const chatSlug = resolvedParams.chatId?.trim() || undefined;

  return <ChatShell activeChatSlug={chatSlug} />;
}
