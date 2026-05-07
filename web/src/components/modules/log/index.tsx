'use client';

import { useCallback, useMemo, useState } from 'react';
import { useLogs, type LogListParams } from '@/api/endpoints/log';
import { LogCard } from './Item';
import { LogHeader, type FilterValues } from './Header';
import { Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';

/**
 * 将 FilterValues 转换为后端 API 参数
 */
function filterToApiParams(filters: FilterValues): LogListParams {
    const now = Math.floor(Date.now() / 1000);
    const rangeSeconds: Record<FilterValues['time_range'], number> = {
        '5m': 300, '30m': 1800, '1h': 3600, '6h': 21600,
        '24h': 86400, '7d': 604800, '30d': 2592000,
    };
    const seconds = rangeSeconds[filters.time_range];
    const start_time = seconds ? now - seconds : undefined;

    return {
        start_time,
        end_time: seconds ? now : undefined,
        model: filters.model || undefined,
        channel_id: filters.channel_id,
        request_id: filters.request_id || undefined,
        status: filters.status === 'all' ? undefined : filters.status,
    };
}

/**
 * 日志页面组件
 */
export function Log() {
    const t = useTranslations('log');
    const [filters, setFilters] = useState<FilterValues>({ time_range: '1h' });

    const apiParams = useMemo(() => filterToApiParams(filters), [filters]);
    const { logs, hasMore, isLoading, isLoadingMore, loadMore } = useLogs({ pageSize: 20, filters: apiParams });

    // 提取唯一模型列表（用于筛选下拉框）
    const models = useMemo(() => {
        const set = new Set(logs.map((l) => l.request_model_name));
        return Array.from(set).sort();
    }, [logs]);

    const handleFilter = useCallback((values: FilterValues) => {
        setFilters(values);
    }, []);

    const canLoadMore = hasMore && !isLoading && !isLoadingMore && logs.length > 0;
    const handleReachEnd = useCallback(() => {
        if (!canLoadMore) return;
        void loadMore();
    }, [canLoadMore, loadMore]);

    const footer = useMemo(() => {
        if (hasMore && (isLoading || isLoadingMore)) {
            return (
                <div className="flex justify-center py-4">
                    <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                </div>
            );
        }
        if (!hasMore && logs.length > 0) {
            return (
                <div className="flex justify-center py-4">
                    <span className="text-sm text-muted-foreground">{t('list.noMore')}</span>
                </div>
            );
        }
        return null;
    }, [hasMore, isLoading, isLoadingMore, logs.length, t]);

    return (
        <div className="flex flex-col h-full">
            <LogHeader
                models={models}
                onFilter={handleFilter}
            />
            <div className="flex-1 min-h-0 overflow-auto px-4 mt-4">
                <VirtualizedGrid
                    items={logs}
                    layout="list"
                    columns={{ default: 1 }}
                    estimateItemHeight={80}
                    overscan={8}
                    getItemKey={(log) => `log-${log.id}`}
                    renderItem={(log) => <LogCard log={log} />}
                    footer={footer}
                    onReachEnd={handleReachEnd}
                    reachEndEnabled={canLoadMore}
                    reachEndOffset={2}
                />
            </div>
        </div>
    );
}
