'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { CheckCircle2, FlaskConical, Loader2, X, XCircle } from 'lucide-react';
import { useTranslations } from 'next-intl';
import type { ModelTestResult } from '@/api/endpoints/channel';

type TestEntry = {
    id: string;
    model: string;
    base_url?: string;
    endpoint_type?: number;
    status: 'pending' | 'done';
    result?: ModelTestResult;
};

type EndpointGroup = {
    base_url: string;
    endpoint_type?: number;
    entries: TestEntry[];
    done: number;
    success: number;
    failed: number;
    avgFirstToken: number;
    avgResponse: number;
};

type StreamEvent = {
    type: 'start' | 'result' | 'done' | 'error';
    tasks?: Array<{ model: string; base_url: string; endpoint_type: number }>;
    skipped?: string[];
    skip_reason?: string;
    result?: ModelTestResult;
    error?: string;
    total?: number;
    success?: number;
    failed?: number;
};

export type TestDialogProps = {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    channelName?: string;
    streamUrl: string;
    body: object | null;
};

const resultKey = (result: Pick<ModelTestResult, 'base_url' | 'model'>) => `${result.base_url || ''}::${result.model}`;

const avg = (values: number[]) => values.length ? Math.round(values.reduce((sum, value) => sum + value, 0) / values.length) : 0;

export function TestDialog({ open, onOpenChange, channelName, streamUrl, body }: TestDialogProps) {
    const t = useTranslations('channel.detail.test');
    const [entries, setEntries] = useState<TestEntry[]>([]);
    const [skipped, setSkipped] = useState<string[]>([]);
    const [skipReason, setSkipReason] = useState<string>('');
    const [isDone, setIsDone] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [summary, setSummary] = useState<{ total: number; success: number; failed: number } | null>(null);
    const abortRef = useRef<AbortController | null>(null);
    const runIdRef = useRef(0);

    useEffect(() => {
        if (!open || !body) return;

        const runId = runIdRef.current + 1;
        runIdRef.current = runId;
        abortRef.current?.abort();

        setEntries([]);
        setSkipped([]);
        setSkipReason('');
        setIsDone(false);
        setError(null);
        setSummary(null);

        const controller = new AbortController();
        abortRef.current = controller;

        const token = typeof window !== 'undefined'
            ? (JSON.parse(localStorage.getItem('auth-storage') || '{}')?.state?.token as string | null)
            : null;

        fetch(streamUrl, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                ...(token ? { Authorization: `Bearer ${token}` } : {}),
            },
            body: JSON.stringify(body),
            signal: controller.signal,
        })
            .then(async (response) => {
                if (runIdRef.current !== runId) return;
                if (!response.ok) {
                    const text = await response.text();
                    try {
                        const json = JSON.parse(text);
                        throw new Error(json.message || text);
                    } catch (e) {
                        if (e instanceof SyntaxError) throw new Error(text || `HTTP ${response.status}`);
                        throw e;
                    }
                }

                const reader = response.body?.getReader();
                if (!reader) throw new Error(t('readStreamFailed'));

                const decoder = new TextDecoder();
                let buffer = '';

                while (true) {
                    const { done, value } = await reader.read();
                    if (runIdRef.current !== runId) return;
                    if (done) break;

                    buffer += decoder.decode(value, { stream: true });
                    const chunks = buffer.split('\n\n');
                    buffer = chunks.pop() || '';

                    for (const chunk of chunks) {
                        const data = chunk
                            .split('\n')
                            .filter((line) => line.startsWith('data:'))
                            .map((line) => line.slice(5).trim())
                            .join('\n');
                        if (!data) continue;

                        try {
                            const event: StreamEvent = JSON.parse(data);
                            if (runIdRef.current !== runId) return;
                            switch (event.type) {
                                case 'start':
                                    setSummary({ total: event.total ?? event.tasks?.length ?? 0, success: 0, failed: 0 });
                                    setEntries(event.tasks?.map((task) => ({
                                        id: `${task.base_url || ''}::${task.model}`,
                                        model: task.model,
                                        base_url: task.base_url,
                                        endpoint_type: task.endpoint_type,
                                        status: 'pending' as const,
                                    })) || []);
                                    if (event.skipped?.length) {
                                        setSkipped(event.skipped);
                                        setSkipReason(event.skip_reason || '');
                                    }
                                    break;
                                case 'result':
                                    if (event.result) {
                                        const id = resultKey(event.result);
                                        setEntries((prev) => {
                                            const next = prev.map((entry) => entry.id === id
                                                ? { id, model: event.result!.model, base_url: event.result!.base_url, status: 'done' as const, result: event.result }
                                                : entry);
                                            if (next.some((entry) => entry.id === id)) return next;
                                            return [...next, { id, model: event.result!.model, base_url: event.result!.base_url, endpoint_type: event.result!.endpoint_type, status: 'done' as const, result: event.result }];
                                        });
                                    }
                                    break;
                                case 'error':
                                    setError(event.error || t('unknownError'));
                                    break;
                                case 'done':
                                    setSummary({ total: event.total ?? 0, success: event.success ?? 0, failed: event.failed ?? 0 });
                                    setIsDone(true);
                                    break;
                            }
                        } catch { /* 忽略单条异常事件，继续读取后续流 */ }
                    }
                }
            })
            .catch((err) => {
                if (runIdRef.current === runId && err.name !== 'AbortError') {
                    setError(err.message || t('requestFailed'));
                }
            });

        return () => { controller.abort(); };
    }, [open, body, streamUrl, t]);

    useEffect(() => {
        if (!open) {
            abortRef.current?.abort();
        }
    }, [open]);

    const stats = useMemo(() => {
        const done = entries.filter((entry) => entry.status === 'done');
        const success = done.filter((entry) => entry.result?.success).length;
        const failed = done.filter((entry) => entry.result && !entry.result.success).length;
        const firstTokenValues = done.map((entry) => entry.result?.first_token_time_ms || 0).filter((value) => value > 0);
        const responseValues = done.map((entry) => entry.result?.response_time_ms || 0).filter((value) => value > 0);
        return {
            done: done.length,
            total: summary?.total || entries.length,
            success: summary?.success ?? success,
            failed: summary?.failed ?? failed,
            avgFirstToken: avg(firstTokenValues),
            avgResponse: avg(responseValues),
        };
    }, [entries, summary]);

    const endpointGroups = useMemo<EndpointGroup[]>(() => {
        const groups = new Map<string, TestEntry[]>();
        for (const entry of entries) {
            const baseUrl = entry.result?.base_url || entry.base_url || t('unknownEndpoint');
            const current = groups.get(baseUrl) || [];
            const next = current.filter((item) => item.id !== entry.id);
            groups.set(baseUrl, [...next, entry]);
        }
        return Array.from(groups.entries()).map(([base_url, groupEntries]) => {
            const done = groupEntries.filter((entry) => entry.status === 'done');
            const firstTokenValues = done.map((entry) => entry.result?.first_token_time_ms || 0).filter((value) => value > 0);
            const responseValues = done.map((entry) => entry.result?.response_time_ms || 0).filter((value) => value > 0);
            return {
                base_url,
                endpoint_type: groupEntries.find((entry) => entry.result?.endpoint_type !== undefined || entry.endpoint_type !== undefined)?.result?.endpoint_type
                    ?? groupEntries.find((entry) => entry.endpoint_type !== undefined)?.endpoint_type,
                entries: groupEntries,
                done: done.length,
                success: done.filter((entry) => entry.result?.success).length,
                failed: done.filter((entry) => entry.result && !entry.result.success).length,
                avgFirstToken: avg(firstTokenValues),
                avgResponse: avg(responseValues),
            };
        });
    }, [entries, t]);

    if (!open) return null;

    const dialog = (
        <div className="fixed inset-0 z-[100] flex items-stretch justify-center p-0 sm:items-center sm:p-4">
            <div
                className="absolute inset-0 bg-white/45 backdrop-blur-xs dark:bg-black/45"
                onClick={() => onOpenChange(false)}
            />
            <div className="relative z-10 flex h-full w-full flex-col overflow-hidden border bg-background shadow-lg sm:h-[min(760px,92vh)] sm:max-w-4xl sm:rounded-2xl">
                <div className="flex items-center justify-between gap-4 border-b px-4 py-3 sm:px-6 sm:py-4">
                    <div className="min-w-0">
                        <h2 className="flex items-center gap-2 text-base font-bold text-card-foreground sm:text-lg">
                            <FlaskConical className="size-5 shrink-0" />
                            <span>{t('title')}</span>
                            {!isDone && !error && <Loader2 className="size-4 animate-spin text-muted-foreground" />}
                        </h2>
                        {channelName && <p className="mt-1 truncate text-xs text-muted-foreground">{channelName}</p>}
                    </div>
                    <button
                        type="button"
                        onClick={() => onOpenChange(false)}
                        className="grid size-8 shrink-0 place-items-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                    >
                        <X className="size-4" />
                    </button>
                </div>

                <div className="grid grid-cols-2 gap-2 border-b px-4 py-3 sm:grid-cols-5 sm:px-6">
                    <Metric label={t('progress')} value={`${stats.done}/${stats.total || 0}`} />
                    <Metric label={t('successLabel')} value={String(stats.success)} tone="success" />
                    <Metric label={t('failedLabel')} value={String(stats.failed)} tone="danger" />
                    <Metric label={t('avgFirstToken')} value={stats.avgFirstToken ? `${stats.avgFirstToken}ms` : '-'} />
                    <Metric label={t('avgResponse')} value={stats.avgResponse ? `${stats.avgResponse}ms` : '-'} />
                </div>

                {skipped.length > 0 && (
                    <div className="border-b bg-muted/30 px-4 py-2 text-xs text-muted-foreground sm:px-6">
                        <span className="font-medium">{skipReason}：</span>
                        <span className="ml-1 font-mono">{skipped.join(', ')}</span>
                    </div>
                )}

                {error && (
                    <div className="border-b bg-destructive/5 px-4 py-3 text-xs text-destructive sm:px-6">
                        {error}
                    </div>
                )}

                <div className="flex-1 overflow-hidden px-4 py-4 sm:px-6">
                    <div className="h-full overflow-y-auto space-y-3">
                        {endpointGroups.map((group) => (
                            <EndpointResultGroup key={group.base_url} group={group} t={t} />
                        ))}
                        {entries.length === 0 && !error && (
                            <div className="flex items-center justify-center rounded-xl border bg-card py-12 text-sm text-muted-foreground">
                                <Loader2 className="mr-2 size-4 animate-spin" />
                                {t('loading')}
                            </div>
                        )}
                    </div>
                </div>
            </div>
        </div>
    );

    return createPortal(dialog, document.body);
}

function Metric({ label, value, tone }: { label: string; value: string; tone?: 'success' | 'danger' }) {
    return (
        <div className="rounded-lg border bg-card px-3 py-2">
            <div className="truncate text-[11px] text-muted-foreground">{label}</div>
            <div className={cn(
                'mt-1 text-sm font-semibold tabular-nums',
                tone === 'success' && 'text-green-600 dark:text-green-400',
                tone === 'danger' && 'text-destructive'
            )}>
                {value}
            </div>
        </div>
    );
}

function EndpointResultGroup({ group, t }: { group: EndpointGroup; t: ReturnType<typeof useTranslations> }) {
    const typeLabel = group.endpoint_type !== undefined ? `Type ${group.endpoint_type}` : '';

    return (
        <section className="overflow-hidden rounded-xl border bg-card">
            <header className="border-b bg-muted/30 px-3 py-2.5 sm:px-4">
                <div className="space-y-1.5">
                    <div className="flex min-w-0 items-center gap-2 text-sm">
                        <span className="shrink-0 font-medium text-muted-foreground">
                            {t('endpoint')}{typeLabel ? `（${typeLabel}）` : ''}：
                        </span>
                        <span className="min-w-0 truncate font-mono text-card-foreground" title={group.base_url}>{group.base_url}</span>
                    </div>

                    <div className="flex flex-wrap items-center gap-x-5 gap-y-1 text-xs text-muted-foreground">
                        <InlineMetric label={t('progress')} value={`${group.done}/${group.entries.length}`} />
                        <InlineMetric label={t('successLabel')} value={String(group.success)} tone="success" />
                        <InlineMetric label={t('avgFirstToken')} value={group.avgFirstToken ? `${group.avgFirstToken}ms` : '-'} />
                        <InlineMetric label={t('avgResponse')} value={group.avgResponse ? `${group.avgResponse}ms` : '-'} />
                    </div>
                </div>
            </header>

            <div className="hidden grid-cols-[minmax(0,1fr)_auto_auto_auto] gap-3 border-b px-3 py-2 text-xs font-medium text-muted-foreground md:grid">
                <span>{t('model')}</span>
                <span>{t('status')}</span>
                <span>{t('firstTokenTime')}</span>
                <span>{t('responseTime')}</span>
            </div>

            {group.entries.map((entry) => <ResultRow key={entry.id} entry={entry} t={t} />)}
        </section>
    );
}

function ResultRow({ entry, t }: { entry: TestEntry; t: ReturnType<typeof useTranslations> }) {
    const failed = entry.status === 'done' && entry.result && !entry.result.success;

    return (
        <div className={cn('border-b last:border-0', failed && 'bg-destructive/5')}>
            <div className="grid gap-2 p-3 md:grid-cols-[minmax(0,1fr)_auto_auto_auto] md:items-center md:gap-3">
                <span className={cn('truncate font-mono text-sm', failed && 'text-destructive')} title={entry.model}>{entry.model}</span>
                <StatusBadge entry={entry} t={t} />
                <span className="text-xs text-muted-foreground md:text-right">
                    {entry.result?.first_token_time_ms ? `${entry.result.first_token_time_ms}ms` : entry.status === 'done' ? '-' : ''}
                </span>
                <span className="text-xs text-muted-foreground md:text-right">
                    {entry.result ? `${entry.result.response_time_ms}ms` : ''}
                </span>
            </div>
            {failed && entry.result?.error && (
                <div className="border-t border-destructive/10 px-3 py-2 md:grid md:grid-cols-[minmax(0,1fr)_auto_auto_auto] md:gap-3">
                    <div className="md:col-span-4">
                        <div className="mb-1 text-[11px] font-medium text-destructive">{t('errorDetail')}</div>
                        <pre className="max-h-24 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-background/70 p-2 font-mono text-xs leading-relaxed text-muted-foreground">
                            {entry.result.error}
                        </pre>
                    </div>
                </div>
            )}
        </div>
    );
}

function InlineMetric({ label, value, tone }: { label: string; value: string; tone?: 'success' }) {
    return (
        <span className="whitespace-nowrap">
            {label}: <span className={cn('font-semibold tabular-nums text-card-foreground', tone === 'success' && 'text-green-600 dark:text-green-400')}>{value}</span>
        </span>
    );
}

function StatusBadge({ entry, t }: { entry: TestEntry; t: ReturnType<typeof useTranslations> }) {
    if (entry.status === 'pending') {
        return (
            <Badge variant="secondary" className="h-6 w-fit gap-1 bg-gray-500/15 px-2 text-xs text-gray-700 dark:text-gray-400">
                <Loader2 className="size-3 animate-spin" />
                {t('pending')}
            </Badge>
        );
    }

    const success = Boolean(entry.result?.success);
    return (
        <Badge
            variant="secondary"
            className={cn(
                'h-6 w-fit gap-1 px-2 text-xs',
                success
                    ? 'bg-green-500/15 text-green-700 dark:text-green-400'
                    : 'bg-red-500/15 text-red-700 dark:text-red-400'
            )}
        >
            {success ? <CheckCircle2 className="size-3" /> : <XCircle className="size-3" />}
            {success ? t('success') : t('failed')}
        </Badge>
    );
}
