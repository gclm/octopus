import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';
import { logger } from '@/lib/logger';
import { formatCount, formatMoney, formatTime } from '@/lib/utils';
import { StatsChannel, type StatsMetricsFormatted } from './stats';

export enum OutboundType {
    OpenAIChat = 0,
    OpenAIResponse = 1,
    Anthropic = 2,
    Gemini = 3,
    OpenAIEmbedding = 5,
}

/**
 * 自动分组类型枚举
 */
export enum AutoGroupType {
    None = 0,   // 不自动分组
    Fuzzy = 1,  // 模糊匹配
    Exact = 2,  // 准确匹配
    Regex = 3,  // 正则匹配
}

export type Endpoint = {
    type: OutboundType;
    base_url: string;
    enabled: boolean;
};

export type CustomHeader = {
    header_key: string;
    header_value: string;
};

export type ChannelKey = {
    id: number;
    channel_id: number;
    enabled: boolean;
    channel_key: string;
    status_code: number;
    last_use_time_stamp: number;
    total_cost: number;
    remark: string;
};

export type Channel = {
    id: number;
    name: string;
    endpoints: Endpoint[];
    keys: ChannelKey[];
    model: string;
    custom_model: string;
    enabled: boolean;
    proxy: boolean;
    auto_sync: boolean;
    auto_group: AutoGroupType;
    custom_header: CustomHeader[];
    param_override?: string | null;
    channel_proxy?: string | null;
    match_regex?: string | null;
    stats: StatsChannel;
};

type ChannelServer = Omit<Channel, 'endpoints' | 'custom_header' | 'keys'> & {
    endpoints: Endpoint[] | null;
    custom_header: CustomHeader[] | null;
    keys: ChannelKey[] | null;
};

export type CreateChannelRequest = {
    name: string;
    endpoints: Endpoint[];
    keys: Array<Pick<ChannelKey, 'enabled' | 'channel_key' | 'remark'>>;
    model: string;
    custom_model?: string;
    enabled?: boolean;
    proxy?: boolean;
    auto_sync?: boolean;
    auto_group?: AutoGroupType;
    custom_header?: CustomHeader[];
    channel_proxy?: string | null;
    param_override?: string | null;
    match_regex?: string | null;
};

export type UpdateChannelRequest = {
    id: number;
    name?: string;
    endpoints?: Endpoint[];
    enabled?: boolean;
    model?: string;
    custom_model?: string;
    proxy?: boolean;
    auto_sync?: boolean;
    auto_group?: AutoGroupType;
    custom_header?: CustomHeader[];
    channel_proxy?: string | null;
    param_override?: string | null;
    match_regex?: string | null;
    keys_to_add?: Array<Pick<ChannelKey, 'enabled' | 'channel_key' | 'remark'>>;
    keys_to_update?: Array<{ id: number; enabled?: boolean; channel_key?: string; remark?: string }>;
    keys_to_delete?: number[];
};

export type FetchModelRequest = {
    endpoints: Endpoint[];
    key: string;
    proxy?: boolean;
    custom_header?: CustomHeader[];
};

export function useChannelList() {
    return useQuery({
        queryKey: ['channels', 'list'],
        queryFn: async () => {
            return apiClient.get<ChannelServer[]>('/api/v1/channel/list');
        },
        select: (data) => data.map((item) => ({
            raw: ({
                ...item,
                endpoints: item.endpoints ?? [],
                custom_header: item.custom_header ?? [],
                keys: item.keys ?? [],
            }) satisfies Channel,
            formatted: {
                input_token: formatCount(item.stats.input_token),
                output_token: formatCount(item.stats.output_token),
                total_token: formatCount(item.stats.input_token + item.stats.output_token),
                input_cost: formatMoney(item.stats.input_cost),
                output_cost: formatMoney(item.stats.output_cost),
                total_cost: formatMoney(item.stats.input_cost + item.stats.output_cost),
                request_success: formatCount(item.stats.request_success),
                request_failed: formatCount(item.stats.request_failed),
                request_count: formatCount(item.stats.request_success + item.stats.request_failed),
                wait_time: formatTime(item.stats.wait_time),
            }
        })) as Array<{ raw: Channel; formatted: StatsMetricsFormatted }>,
        refetchInterval: 30000,
        refetchOnMount: 'always',
    });
}

export function useCreateChannel() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (data: CreateChannelRequest) => {
            return apiClient.post<ChannelServer>('/api/v1/channel/create', data);
        },
        onSuccess: (data) => {
            logger.log('渠道创建成功:', data);
            queryClient.invalidateQueries({ queryKey: ['channels', 'list'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'list'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'channel'] });
        },
        onError: (error) => {
            logger.error('渠道创建失败:', error);
        },
    });
}

export function useUpdateChannel() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (data: UpdateChannelRequest) => {
            return apiClient.post<ChannelServer>('/api/v1/channel/update', data);
        },
        onSuccess: (data) => {
            logger.log('渠道更新成功:', data);
            queryClient.invalidateQueries({ queryKey: ['channels', 'list'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'channel'] });
        },
        onError: (error) => {
            logger.error('渠道更新失败:', error);
        },
    });
}

export function useDeleteChannel() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (id: number) => {
            return apiClient.delete<null>(`/api/v1/channel/delete/${id}`);
        },
        onSuccess: () => {
            logger.log('渠道删除成功');
            queryClient.invalidateQueries({ queryKey: ['channels', 'list'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'channel'] });
        },
        onError: (error) => {
            logger.error('渠道删除失败:', error);
        },
    });
}

export function useEnableChannel() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (data: { id: number; enabled: boolean }) => {
            return apiClient.post<null>('/api/v1/channel/enable', data);
        },
        onSuccess: () => {
            logger.log('渠道状态更新成功');
            queryClient.invalidateQueries({ queryKey: ['channels', 'list'] });
        },
        onError: (error) => {
            logger.error('渠道状态更新失败:', error);
        },
    });
}

export function useFetchModel() {
    return useMutation({
        mutationFn: async (data: FetchModelRequest) => {
            return apiClient.post<string[]>('/api/v1/channel/fetch-model', data);
        },
        onSuccess: (data) => {
            logger.log('模型列表获取成功:', data);
        },
        onError: (error) => {
            logger.error('模型列表获取失败:', error);
        },
    });
}

export function useLastSyncTime() {
    return useQuery({
        queryKey: ['channels', 'last-sync-time'],
        queryFn: async () => {
            return apiClient.get<string>('/api/v1/channel/last-sync-time');
        },
        refetchInterval: 30000,
    });
}

export function useSyncChannel() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async () => {
            return apiClient.post<null>('/api/v1/channel/sync');
        },
        onSuccess: () => {
            logger.log('渠道同步成功');
            queryClient.invalidateQueries({ queryKey: ['channels', 'last-sync-time'] });
        },
        onError: (error) => {
            logger.error('渠道同步失败:', error);
        },
    });
}
