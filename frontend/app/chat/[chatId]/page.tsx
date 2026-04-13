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
  const parsed = Number(resolvedParams.chatId);
  const chatId = Number.isFinite(parsed) ? parsed : undefined;

  return <ChatShell activeChatId={chatId} />;
}
