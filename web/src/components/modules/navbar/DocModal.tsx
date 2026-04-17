'use client';

import { useState, useMemo, useEffect } from 'react';
import { useTranslations } from 'next-intl';
import { Copy, Check, BookOpen, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useGroupList } from '@/api/endpoints/group';
import { useAPIKeyList } from '@/api/endpoints/apikey';
import { API_BASE_URL } from '@/api/client';
import { motion, AnimatePresence } from 'motion/react';

type ApiType = 'openai-chat' | 'openai-responses' | 'anthropic';

const API_PATHS: Record<ApiType, string> = {
    'openai-chat': '/v1/chat/completions',
    'openai-responses': '/v1/responses',
    'anthropic': '/v1/messages',
};

function generateCurl(baseUrl: string, apiKey: string, model: string, apiType: ApiType): string {
    const path = API_PATHS[apiType];
    const url = `${baseUrl}${path}`;

    if (apiType === 'anthropic') {
        return `curl -X POST '${url}' \\
  -H 'Content-Type: application/json' \\
  -H 'x-api-key: ${apiKey || 'YOUR_API_KEY'}' \\
  -H 'anthropic-version: 2023-06-01' \\
  -d '{
    "model": "${model || 'YOUR_MODEL'}",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'`;
    }

    if (apiType === 'openai-responses') {
        return `curl -X POST '${url}' \\
  -H 'Content-Type: application/json' \\
  -H 'Authorization: Bearer ${apiKey || 'YOUR_API_KEY'}' \\
  -d '{
    "model": "${model || 'YOUR_MODEL'}",
    "input": "Hello!"
  }'`;
    }

    return `curl -X POST '${url}' \\
  -H 'Content-Type: application/json' \\
  -H 'Authorization: Bearer ${apiKey || 'YOUR_API_KEY'}' \\
  -d '{
    "model": "${model || 'YOUR_MODEL'}",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'`;
}

interface DocModalProps {
    isOpen: boolean;
    onClose: () => void;
    onGoSetting?: () => void;
}

export function DocModal({ isOpen, onClose }: DocModalProps) {
    const t = useTranslations('doc');
    const [apiType, setApiType] = useState<ApiType>('openai-chat');
    const [selectedApiKey, setSelectedApiKey] = useState<string>('');
    const [selectedModel, setSelectedModel] = useState<string>('');
    const [copied, setCopied] = useState(false);

    const { data: groups } = useGroupList();
    const { data: apiKeys } = useAPIKeyList();

    const baseUrl = useMemo(() => {
        if (typeof window === 'undefined') return '';
        const configured = API_BASE_URL && API_BASE_URL !== '.' ? API_BASE_URL : '';
        return configured || window.location.origin;
    }, []);

    const curlCode = useMemo(
        () => generateCurl(baseUrl, selectedApiKey, selectedModel, apiType),
        [baseUrl, selectedApiKey, selectedModel, apiType]
    );

    const groupOptions = useMemo(() => {
        return Array.from(new Set((groups ?? []).map((g) => g.name).filter(Boolean)));
    }, [groups]);

    useEffect(() => {
        if (selectedModel && !groupOptions.includes(selectedModel)) {
            setSelectedModel('');
        }
    }, [groupOptions, selectedModel]);

    const handleCopy = async () => {
        try {
            await navigator.clipboard.writeText(curlCode);
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
        } catch {
            // fallback
        }
    };

    return (
        <AnimatePresence>
            {isOpen && (
                <>
                    <motion.div
                        className="fixed inset-0 bg-black/40 z-50"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        onClick={onClose}
                    />
                    <motion.div
                        className="fixed inset-x-4 bottom-4 top-4 md:inset-auto md:left-1/2 md:top-1/2 md:-translate-x-1/2 md:-translate-y-1/2 md:w-[640px] md:max-h-[80vh] z-50 flex flex-col bg-card rounded-3xl border border-border shadow-2xl overflow-hidden"
                        initial={{ opacity: 0, scale: 0.95, y: 20 }}
                        animate={{ opacity: 1, scale: 1, y: 0 }}
                        exit={{ opacity: 0, scale: 0.95, y: 20 }}
                        transition={{ type: 'spring', stiffness: 300, damping: 30 }}
                    >
                        <div className="flex items-center justify-between p-6 border-b border-border shrink-0">
                            <div className="flex items-center gap-2">
                                <BookOpen className="h-5 w-5 text-primary" />
                                <h2 className="text-lg font-bold text-card-foreground">{t('title')}</h2>
                            </div>
                            <button
                                onClick={onClose}
                                className="p-1.5 rounded-xl text-muted-foreground hover:text-card-foreground hover:bg-muted/50 transition-colors"
                            >
                                <X className="h-5 w-5" />
                            </button>
                        </div>

                        <div className="flex-1 overflow-y-auto p-6 space-y-5">
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                <div className="space-y-1">
                                    <label className="text-sm font-medium text-muted-foreground">{t('baseUrl')}</label>
                                    <div className="font-mono text-sm bg-muted/30 rounded-xl px-3 py-2 text-card-foreground break-all truncate">{baseUrl}</div>
                                </div>
                                <div className="space-y-1">
                                    <label className="text-sm font-medium text-muted-foreground">{t('apiType')}</label>
                                    <Select value={apiType} onValueChange={(v) => setApiType(v as ApiType)}>
                                        <SelectTrigger className="rounded-xl">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent className="rounded-xl">
                                            <SelectItem value="openai-chat">OpenAI Chat</SelectItem>
                                            <SelectItem value="openai-responses">OpenAI Responses</SelectItem>
                                            <SelectItem value="anthropic">Anthropic</SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>
                            </div>

                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                <div className="space-y-1">
                                    <label className="text-sm font-medium text-muted-foreground">{t('apiKey')}</label>
                                    <Select value={selectedApiKey} onValueChange={setSelectedApiKey}>
                                        <SelectTrigger className="rounded-xl">
                                            <SelectValue placeholder={t('selectApiKey')} />
                                        </SelectTrigger>
                                        <SelectContent className="rounded-xl">
                                            {(apiKeys ?? []).map((key) => (
                                                <SelectItem key={key.id} value={key.api_key}>
                                                    {key.name || key.api_key.slice(0, 12) + '...'}
                                                </SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                </div>
                                <div className="space-y-1">
                                    <label className="text-sm font-medium text-muted-foreground">{t('model')}</label>
                                    <Select value={selectedModel} onValueChange={setSelectedModel}>
                                        <SelectTrigger className="rounded-xl">
                                            <SelectValue placeholder={t('selectModel')} />
                                        </SelectTrigger>
                                        <SelectContent className="rounded-xl">
                                            {groupOptions.map((name) => (
                                                <SelectItem key={name} value={name}>{name}</SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                </div>
                            </div>

                            <div className="space-y-2">
                                <div className="flex items-center justify-between">
                                    <label className="text-sm font-medium text-muted-foreground">{t('curl')}</label>
                                    <Button type="button" variant="ghost" size="sm" onClick={handleCopy} className="rounded-xl">
                                        {copied ? <Check className="h-4 w-4 mr-1" /> : <Copy className="h-4 w-4 mr-1" />}
                                        {copied ? t('copied') : t('copy')}
                                    </Button>
                                </div>
                                <pre className="bg-muted/30 rounded-xl p-4 text-xs font-mono overflow-x-auto whitespace-pre-wrap break-all text-card-foreground">
                                    {curlCode}
                                </pre>
                            </div>
                        </div>
                    </motion.div>
                </>
            )}
        </AnimatePresence>
    );
}
