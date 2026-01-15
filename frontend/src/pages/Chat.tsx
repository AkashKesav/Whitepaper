import React, { useState, useRef, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { useAuth } from '@/contexts/AuthContext';
import { cn } from '@/lib/utils';
import {
    Send, Plus, Search, MoreHorizontal, Sparkles,
    MessageCircle, Users, Archive, Settings, ChevronLeft
} from 'lucide-react';

interface Message {
    id: string;
    role: 'user' | 'assistant';
    content: string;
    timestamp: Date;
    memoryContext?: string[];
}

interface Conversation {
    id: string;
    title?: string;
    last_message?: string;
    created_at?: string;
    updated_at?: string;
    // For UI display
    timestamp?: Date;
    unread?: boolean;
}

export const Chat: React.FC = () => {
    const { user } = useAuth();
    const queryClient = useQueryClient();

    // Fetch conversations from API
    const { data: conversationsData, isLoading: conversationsLoading, refetch: refetchConversations } = useQuery({
        queryKey: ['conversations'],
        queryFn: () => api.getConversations(),
        retry: false,
    });

    const conversations = conversationsData?.conversations || [];

    const [selectedConversation, setSelectedConversation] = useState<string | null>(null);
    const [messages, setMessages] = useState<Message[]>([
        { id: '1', role: 'assistant', content: 'Hello! I\'m your AI assistant with persistent memory. How can I help you today?', timestamp: new Date() },
    ]);
    const [inputValue, setInputValue] = useState('');
    const [isTyping, setIsTyping] = useState(false);
    const [searchQuery, setSearchQuery] = useState('');
    const messagesEndRef = useRef<HTMLDivElement>(null);

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    };

    useEffect(() => {
        scrollToBottom();
    }, [messages]);

    const handleSend = async () => {
        if (!inputValue.trim()) return;

        const userMessage: Message = {
            id: Date.now().toString(),
            role: 'user',
            content: inputValue,
            timestamp: new Date(),
        };

        setMessages(prev => [...prev, userMessage]);
        const messageToSend = inputValue;
        setInputValue('');
        setIsTyping(true);

        try {
            const data = await api.sendMessage(messageToSend, selectedConversation || undefined);

            const aiMessage: Message = {
                id: (Date.now() + 1).toString(),
                role: 'assistant',
                content: data.response || 'Sorry, I could not generate a response.',
                timestamp: new Date(),
                memoryContext: data.context_used ? ['Memory context applied'] : undefined,
            };
            setMessages(prev => [...prev, aiMessage]);

            // If new conversation, set ID
            if (data.conversation_id && data.conversation_id !== selectedConversation) {
                setSelectedConversation(data.conversation_id);
            }
            // Refetch conversations to show latest
            refetchConversations();
        } catch (error) {
            console.error('Chat error:', error);
            const errorMessage: Message = {
                id: (Date.now() + 1).toString(),
                role: 'assistant',
                content: 'Sorry, there was an error processing your request. Please make sure you are logged in.',
                timestamp: new Date(),
            };
            setMessages(prev => [...prev, errorMessage]);
        } finally {
            setIsTyping(false);
        }
    };

    const filteredConversations = conversations.filter((c: Conversation) =>
        (c.title || 'New Conversation').toLowerCase().includes(searchQuery.toLowerCase())
    );

    return (
        <div className="h-screen flex bg-[#1C1C1E]">
            {/* Sidebar */}
            <div
                className="w-72 flex flex-col border-r"
                style={{
                    background: 'rgba(28, 28, 30, 0.95)',
                    borderColor: 'rgba(255,255,255,0.1)'
                }}
            >
                {/* Header */}
                <div className="p-4 border-b border-white/10">
                    <div className="flex items-center justify-between mb-4">
                        <h1 className="text-lg font-semibold text-white">Messages</h1>
                        <button className="p-2 rounded-lg hover:bg-white/10 transition-colors">
                            <Plus className="w-5 h-5 text-white/60" />
                        </button>
                    </div>
                    <div className="relative">
                        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-white/40" />
                        <input
                            type="text"
                            placeholder="Search conversations"
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            className="w-full pl-10 pr-4 py-2 bg-white/5 rounded-xl text-sm text-white placeholder-white/40 outline-none focus:bg-white/10 transition-colors"
                        />
                    </div>
                </div>

                {/* Conversation List */}
                <div className="flex-1 overflow-y-auto p-2">
                    {conversationsLoading ? (
                        <div className="text-center text-white/50 py-8">Loading conversations...</div>
                    ) : filteredConversations.length === 0 ? (
                        <div className="text-center text-white/50 py-8">
                            <p>No conversations yet</p>
                            <p className="text-xs mt-2">Start a new conversation below</p>
                        </div>
                    ) : (
                        filteredConversations.map((conv: Conversation) => (
                            <button
                                key={conv.id}
                                onClick={() => setSelectedConversation(conv.id)}
                                className={cn(
                                    "w-full p-3 rounded-xl text-left transition-all mb-1",
                                    selectedConversation === conv.id
                                        ? "bg-white/10"
                                        : "hover:bg-white/5"
                                )}
                            >
                                <div className="flex items-start gap-3">
                                    <div className="w-10 h-10 rounded-full bg-gradient-to-br from-purple-500 to-blue-500 flex items-center justify-center flex-shrink-0">
                                        <MessageCircle className="w-5 h-5 text-white" />
                                    </div>
                                    <div className="flex-1 min-w-0">
                                        <div className="flex items-center justify-between">
                                            <span className={cn("font-medium text-sm", conv.unread ? "text-white" : "text-white/80")}>
                                                {conv.title || 'New Conversation'}
                                            </span>
                                            {conv.unread && (
                                                <span className="w-2 h-2 rounded-full bg-blue-500" />
                                            )}
                                        </div>
                                        <p className="text-xs text-white/50 truncate mt-0.5">
                                            {conv.last_message || 'Start chatting...'}
                                        </p>
                                    </div>
                                </div>
                            </button>
                        ))
                    )}
                </div>

                {/* Bottom Nav */}
                <div className="p-3 border-t border-white/10 flex justify-around">
                    <button className="p-2 rounded-lg bg-white/10 text-white">
                        <MessageCircle className="w-5 h-5" />
                    </button>
                    <button className="p-2 rounded-lg hover:bg-white/10 text-white/50 transition-colors">
                        <Users className="w-5 h-5" />
                    </button>
                    <button className="p-2 rounded-lg hover:bg-white/10 text-white/50 transition-colors">
                        <Archive className="w-5 h-5" />
                    </button>
                    <button className="p-2 rounded-lg hover:bg-white/10 text-white/50 transition-colors">
                        <Settings className="w-5 h-5" />
                    </button>
                </div>
            </div>

            {/* Main Chat Area */}
            <div className="flex-1 flex flex-col bg-black">
                {/* Chat Header */}
                <div className="h-14 border-b border-white/10 flex items-center justify-between px-4">
                    <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-full bg-gradient-to-br from-purple-500 to-blue-500" />
                        <div>
                            <h2 className="text-sm font-medium text-white">Project Planning</h2>
                            <p className="text-xs text-white/50">Memory-enhanced</p>
                        </div>
                    </div>
                    <div className="flex items-center gap-2">
                        <button className="p-2 rounded-lg hover:bg-white/10 transition-colors">
                            <Sparkles className="w-4 h-4 text-white/60" />
                        </button>
                        <button className="p-2 rounded-lg hover:bg-white/10 transition-colors">
                            <MoreHorizontal className="w-4 h-4 text-white/60" />
                        </button>
                    </div>
                </div>

                {/* Messages */}
                <div className="flex-1 overflow-y-auto p-4 space-y-4">
                    {messages.map((msg) => (
                        <div
                            key={msg.id}
                            className={cn(
                                "flex",
                                msg.role === 'user' ? "justify-end" : "justify-start"
                            )}
                        >
                            <div
                                className={cn(
                                    "max-w-[70%] rounded-2xl px-4 py-3",
                                    msg.role === 'user'
                                        ? "bg-blue-500 text-white"
                                        : "bg-white/10 text-white/90"
                                )}
                            >
                                <p className="text-sm">{msg.content}</p>
                                {msg.memoryContext && msg.memoryContext.length > 0 && (
                                    <div className="mt-2 pt-2 border-t border-white/20">
                                        <p className="text-xs text-white/50 flex items-center gap-1">
                                            <Sparkles className="w-3 h-3" /> Memory context used
                                        </p>
                                    </div>
                                )}
                            </div>
                        </div>
                    ))}
                    {isTyping && (
                        <div className="flex justify-start">
                            <div className="bg-white/10 rounded-2xl px-4 py-3">
                                <div className="flex gap-1">
                                    <span className="w-2 h-2 bg-white/50 rounded-full animate-bounce" />
                                    <span className="w-2 h-2 bg-white/50 rounded-full animate-bounce" style={{ animationDelay: '0.1s' }} />
                                    <span className="w-2 h-2 bg-white/50 rounded-full animate-bounce" style={{ animationDelay: '0.2s' }} />
                                </div>
                            </div>
                        </div>
                    )}
                    <div ref={messagesEndRef} />
                </div>

                {/* Input */}
                <div className="p-4 border-t border-white/10">
                    <div className="flex items-center gap-3 bg-white/5 rounded-2xl px-4 py-3">
                        <input
                            type="text"
                            value={inputValue}
                            onChange={(e) => setInputValue(e.target.value)}
                            onKeyPress={(e) => e.key === 'Enter' && handleSend()}
                            placeholder="Message..."
                            className="flex-1 bg-transparent text-white placeholder-white/40 outline-none text-sm"
                        />
                        <button
                            onClick={handleSend}
                            disabled={!inputValue.trim()}
                            className={cn(
                                "p-2 rounded-full transition-all",
                                inputValue.trim()
                                    ? "bg-blue-500 text-white"
                                    : "bg-white/10 text-white/30"
                            )}
                        >
                            <Send className="w-4 h-4" />
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default Chat;
