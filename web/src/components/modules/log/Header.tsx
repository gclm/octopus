'use client';

import { useState, useMemo } from 'react';
import { useChannelList } from '@/api/endpoints/channel';
import { useTranslations } from 'next-intl';
import { Search, Check, X, Clock } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
    Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select';
import { cn } from '@/lib/utils';

export interface FilterValues {
    model?: string;
    channel_id?: number;
    status?: 'all' | 'success' | 'error';
    request_id?: string;
    time_range: '5m' | '30m' | '1h' | '6h' | '24h' | '7d' | '30d';
}

interface LogHeaderProps {
    models: string[];
    onFilter: (values: FilterValues) => void;
}

const TIME_RANGES: { value: FilterValues['time_range']; label: string }[] = [
    { value: '5m', label: 'log.filter.last5m' },
    { value: '30m', label: 'log.filter.last30m' },
    { value: '1h', label: 'log.filter.last1h' },
    { value: '6h', label: 'log.filter.last6h' },
    { value: '24h', label: 'log.filter.last24h' },
    { value: '7d', label: 'log.filter.last7d' },
    { value: '30d', label: 'log.filter.last30d' },
];

export function LogHeader({ models, onFilter }: LogHeaderProps) {
    const t = useTranslations();
    const { data: channels } = useChannelList();
    const [filters, setFilters] = useState<FilterValues>({ time_range: '1h', status: 'all' });
    const [keyword, setKeyword] = useState('');

    const activeFilterCount = useMemo(() => {
        let count = 0;
        if (filters.time_range !== '1h') count++;
        if (filters.status && filters.status !== 'all') count++;
        if (filters.model) count++;
        if (filters.channel_id) count++;
        if (keyword) count++;
        return count;
    }, [filters, keyword]);

    const handleApply = () => {
        onFilter({
            ...filters,
            request_id: keyword || undefined,
        });
    };

    const handleReset = () => {
        setFilters({ time_range: '1h', status: 'all' });
        setKeyword('');
        onFilter({ time_range: '1h', status: 'all' });
    };

    return (
        <div className="rounded-3xl border border-border bg-card mx-4 mt-4">
            <div className="flex flex-wrap items-center gap-2 px-4 py-3">
                {/* 搜索框 */}
                <div className="relative w-52">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                    <Input
                        placeholder={t('log.filter.searchPlaceholder')}
                        value={keyword}
                        onChange={(e) => setKeyword(e.target.value)}
                        className="pl-9 h-8 text-sm rounded-xl border-border/70 focus-visible:ring-primary/20"
                        onKeyDown={(e) => e.key === 'Enter' && handleApply()}
                    />
                </div>

                {/* 时间范围 */}
                <Select
                    value={filters.time_range}
                    onValueChange={(v) => setFilters({ ...filters, time_range: v as FilterValues['time_range'] })}
                >
                    <SelectTrigger className={cn(
                        "h-8 w-36 rounded-xl text-xs border-border/70",
                        filters.time_range !== '1h' && "border-primary/30 bg-primary/5 text-primary"
                    )}>
                        <Clock className="h-3.5 w-3.5 mr-1.5 shrink-0 text-muted-foreground" />
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {TIME_RANGES.map((r) => (
                            <SelectItem key={r.value} value={r.value}>{t(r.label)}</SelectItem>
                        ))}
                    </SelectContent>
                </Select>

                {/* 状态 */}
                <Select
                    value={filters.status}
                    onValueChange={(v) => setFilters({ ...filters, status: v as FilterValues['status'] })}
                >
                    <SelectTrigger className={cn(
                        "h-8 w-28 rounded-xl text-xs border-border/70",
                        filters.status && filters.status !== 'all' && "border-primary/30 bg-primary/5 text-primary"
                    )}>
                        <SelectValue placeholder={t('log.filter.status')} />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="all">{t('log.filter.all')}</SelectItem>
                        <SelectItem value="success">{t('log.filter.success')}</SelectItem>
                        <SelectItem value="error">{t('log.filter.error')}</SelectItem>
                    </SelectContent>
                </Select>

                {/* 模型 */}
                {models.length > 0 && (
                    <Select
                        value={filters.model || '__all__'}
                        onValueChange={(v) => setFilters({ ...filters, model: v === '__all__' ? undefined : v })}
                    >
                        <SelectTrigger className={cn(
                            "h-8 w-36 rounded-xl text-xs border-border/70",
                            filters.model && "border-primary/30 bg-primary/5 text-primary"
                        )}>
                            <SelectValue placeholder={t('log.filter.model')} />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="__all__">{t('log.filter.all')}</SelectItem>
                            {models.map((m) => (
                                <SelectItem key={m} value={m}>{m}</SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                )}

                {/* 渠道 */}
                {channels && channels.length > 0 && (
                    <Select
                        value={filters.channel_id ? filters.channel_id.toString() : '__all__'}
                        onValueChange={(v) => setFilters({ ...filters, channel_id: v === '__all__' ? undefined : Number(v) })}
                    >
                        <SelectTrigger className={cn(
                            "h-8 w-36 rounded-xl text-xs border-border/70",
                            filters.channel_id && "border-primary/30 bg-primary/5 text-primary"
                        )}>
                            <SelectValue placeholder={t('log.filter.channel')} />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="__all__">{t('log.filter.all')}</SelectItem>
                            {channels.map((ch) => (
                                <SelectItem key={ch.raw.id} value={ch.raw.id.toString()}>{ch.raw.name}</SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                )}

                {/* 操作按钮 */}
                <div className="ml-auto flex items-center gap-2">
                    {activeFilterCount > 0 && (
                        <Button
                            size="sm"
                            variant="ghost"
                            className="h-8 px-2 text-xs text-muted-foreground hover:text-foreground rounded-xl"
                            onClick={handleReset}
                        >
                            <X className="h-3.5 w-3.5 mr-1" />
                            {t('log.filter.reset')}
                        </Button>
                    )}
                    <Button
                        size="sm"
                        className="h-8 px-4 text-xs rounded-xl"
                        onClick={handleApply}
                    >
                        <Check className="h-3.5 w-3.5 mr-1" />
                        {t('log.filter.apply')}
                    </Button>
                </div>
            </div>
        </div>
    );
}
