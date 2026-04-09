import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../client';
import { formatCount, formatMoney, formatTime } from '@/lib/utils';

/**
 * 统计数据
 */
interface StatsMetrics {
    input_token: number;
    output_token: number;
    input_cost: number;
    output_cost: number;
    wait_time: number;
    request_success: number;
    request_failed: number;
    cached_tokens: number;
    cached_cost: number;
}

export interface StatsMetricsFormatted {
    input_token: ReturnType<typeof formatCount>;
    output_token: ReturnType<typeof formatCount>;
    input_cost: ReturnType<typeof formatMoney>;
    output_cost: ReturnType<typeof formatMoney>;
    wait_time: ReturnType<typeof formatTime>;
    request_success: ReturnType<typeof formatCount>;
    request_failed: ReturnType<typeof formatCount>;

    request_count: ReturnType<typeof formatCount>;
    total_token: ReturnType<typeof formatCount>;
    total_cost: ReturnType<typeof formatMoney>;
}

export interface StatsChannel extends StatsMetrics {
    channel_id: number;
}

export interface StatsDaily extends StatsMetrics {
    date: string;
}
export interface StatsDailyFormatted extends StatsMetricsFormatted {
    date: string;
}

export interface StatsTotal extends StatsMetrics {
    id: number;
}
export type StatsTotalFormatted = StatsMetricsFormatted;

export interface StatsHourly extends StatsMetrics {
    hour: number;
    date: string;
}
export interface StatsHourlyFormatted extends StatsMetricsFormatted {
    hour: number;
    date: string;
}
/**
 * API Key 统计数据
 */
export interface StatsAPIKey extends StatsMetrics {
    api_key_id: number;
}

export interface StatsAPIKeyFormatted extends StatsMetricsFormatted {
    api_key_id: number;
}
/**
 * 获取今日统计数据 Hook
 */
export function useStatsToday() {
    return useQuery({
        queryKey: ['stats', 'today'],
        queryFn: async () => {
            return apiClient.get<StatsDaily>('/api/v1/stats/today');
        },
        refetchInterval: 30000,
        refetchOnMount: 'always',
    });
}

/**
 * 获取每日统计数据 Hook
 */
export function useStatsDaily() {
    return useQuery({
        queryKey: ['stats', 'daily'],
        queryFn: async () => {
            return apiClient.get<StatsDaily[]>('/api/v1/stats/daily');
        },
        select: (data) => data.map((item): StatsDailyFormatted => ({
            input_token: formatCount(item.input_token),
            output_token: formatCount(item.output_token),
            total_token: formatCount(item.input_token + item.output_token),
            input_cost: formatMoney(item.input_cost),
            output_cost: formatMoney(item.output_cost),
            total_cost: formatMoney(item.input_cost + item.output_cost),
            wait_time: formatTime(item.wait_time),
            request_success: formatCount(item.request_success),
            request_failed: formatCount(item.request_failed),
            request_count: formatCount(item.request_success + item.request_failed),
            date: item.date,
        })),
        refetchInterval: 3600000, // 1 小时
        refetchOnMount: 'always',
    });
}
/**
 * 获取总统计数据 Hook
 */
export function useStatsHourly() {
    return useQuery({
        queryKey: ['stats', 'hourly'],
        queryFn: async () => {
            return apiClient.get<StatsHourly[]>('/api/v1/stats/hourly');
        },
        select: (data) => data.map((item): StatsHourlyFormatted => ({
            hour: item.hour,
            date: item.date,
            input_token: formatCount(item.input_token),
            output_token: formatCount(item.output_token),
            total_token: formatCount(item.input_token + item.output_token),
            input_cost: formatMoney(item.input_cost),
            output_cost: formatMoney(item.output_cost),
            total_cost: formatMoney(item.input_cost + item.output_cost),
            wait_time: formatTime(item.wait_time),
            request_success: formatCount(item.request_success),
            request_failed: formatCount(item.request_failed),
            request_count: formatCount(item.request_success + item.request_failed),
        })),
        refetchInterval: 10000,// 10 秒
        refetchOnMount: 'always',
    });
}

export function useStatsTotal() {
    return useQuery({
        queryKey: ['stats', 'total'],
        queryFn: async () => {
            return apiClient.get<StatsTotal>('/api/v1/stats/total');
        },
        select: (data) => ({
            input_token: formatCount(data.input_token),
            output_token: formatCount(data.output_token),
            total_token: formatCount(data.input_token + data.output_token),
            input_cost: formatMoney(data.input_cost),
            output_cost: formatMoney(data.output_cost),
            total_cost: formatMoney(data.input_cost + data.output_cost),
            wait_time: formatTime(data.wait_time),
            request_success: formatCount(data.request_success),
            request_failed: formatCount(data.request_failed),
            request_count: formatCount(data.request_success + data.request_failed),
        }),
        refetchInterval: 10000,// 10 秒
        refetchOnMount: 'always',
    });
}



/**
 * 获取 API Key 统计数据列表 Hook
 */
export function useStatsAPIKey() {
    return useQuery({
        queryKey: ['stats', 'apikey'],
        queryFn: async () => {
            return apiClient.get<StatsAPIKey[]>('/api/v1/stats/apikey');
        },
        select: (data) => data.map((item): StatsAPIKeyFormatted => ({
            api_key_id: item.api_key_id,
            input_token: formatCount(item.input_token),
            output_token: formatCount(item.output_token),
            total_token: formatCount(item.input_token + item.output_token),
            input_cost: formatMoney(item.input_cost),
            output_cost: formatMoney(item.output_cost),
            total_cost: formatMoney(item.input_cost + item.output_cost),
            wait_time: formatTime(item.wait_time),
            request_success: formatCount(item.request_success),
            request_failed: formatCount(item.request_failed),
            request_count: formatCount(item.request_success + item.request_failed),
        })),
        refetchInterval: 30000,
        refetchOnMount: 'always',
    });
}

/**
 * 按时间范围查询聚合统计
 */
export interface StatsRangeResponse {
    request_count: number;
    request_success: number;
    request_failed: number;
    success_rate: number;
    avg_response_time: number;

    total_tokens: number;
    input_tokens: number;
    output_tokens: number;
    cached_tokens: number;
    cache_hit_rate: number;

    total_cost: number;
    input_cost: number;
    output_cost: number;
    cached_cost: number;
    cost_saved: number;
}

export interface StatsRangeResponseFormatted {
    request_count: ReturnType<typeof formatCount>;
    request_success: ReturnType<typeof formatCount>;
    request_failed: ReturnType<typeof formatCount>;
    success_rate: string;
    avg_response_time: ReturnType<typeof formatTime>;

    total_tokens: ReturnType<typeof formatCount>;
    input_tokens: ReturnType<typeof formatCount>;
    output_tokens: ReturnType<typeof formatCount>;
    cached_tokens: ReturnType<typeof formatCount>;
    cache_hit_rate: string;

    total_cost: ReturnType<typeof formatMoney>;
    input_cost: ReturnType<typeof formatMoney>;
    output_cost: ReturnType<typeof formatMoney>;
    cached_cost: ReturnType<typeof formatMoney>;
    cost_saved: ReturnType<typeof formatMoney>;
}

export function useStatsRange(startDate: string, endDate: string) {
    return useQuery({
        queryKey: ['stats', 'range', startDate, endDate],
        queryFn: async () => {
            return apiClient.get<StatsRangeResponse>(`/api/v1/stats/range?start_date=${startDate}&end_date=${endDate}`);
        },
        select: (data): StatsRangeResponseFormatted => ({
            request_count: formatCount(data.request_count),
            request_success: formatCount(data.request_success),
            request_failed: formatCount(data.request_failed),
            success_rate: `${data.success_rate.toFixed(1)}%`,
            avg_response_time: formatTime(data.avg_response_time),

            total_tokens: formatCount(data.total_tokens),
            input_tokens: formatCount(data.input_tokens),
            output_tokens: formatCount(data.output_tokens),
            cached_tokens: formatCount(data.cached_tokens),
            cache_hit_rate: `${data.cache_hit_rate.toFixed(1)}%`,

            total_cost: formatMoney(data.total_cost),
            input_cost: formatMoney(data.input_cost),
            output_cost: formatMoney(data.output_cost),
            cached_cost: formatMoney(data.cached_cost),
            cost_saved: formatMoney(data.cost_saved),
        }),
        enabled: !!startDate && !!endDate,
        refetchInterval: 60000, // 1 分钟
        refetchOnMount: 'always',
    });
}

/**
 * 按时间范围查询模型统计
 */
export interface StatsModelItem {
    name: string;
    request_count: number;
    total_tokens: number;
    total_cost: number;
    percentage: number;
}

export interface StatsModelItemFormatted {
    name: string;
    request_count: ReturnType<typeof formatCount>;
    total_tokens: ReturnType<typeof formatCount>;
    total_cost: ReturnType<typeof formatMoney>;
    percentage: string;
}

export function useStatsModels(startDate: string, endDate: string) {
    return useQuery({
        queryKey: ['stats', 'models', startDate, endDate],
        queryFn: async () => {
            return apiClient.get<StatsModelItem[]>(`/api/v1/stats/models?start_date=${startDate}&end_date=${endDate}`);
        },
        select: (data) => data.map((item): StatsModelItemFormatted => ({
            name: item.name,
            request_count: formatCount(item.request_count),
            total_tokens: formatCount(item.total_tokens),
            total_cost: formatMoney(item.total_cost),
            percentage: `${item.percentage.toFixed(1)}%`,
        })),
        enabled: !!startDate && !!endDate,
        refetchInterval: 60000, // 1 分钟
        refetchOnMount: 'always',
    });
}