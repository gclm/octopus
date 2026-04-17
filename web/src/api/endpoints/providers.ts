import { apiClient } from '../client';
import { useQuery } from '@tanstack/react-query';

export interface Provider {
    name: string;
    channel_type: number;
    base_url: string;
    auth_type?: string; // 'oauth_device' | undefined (default: api_key)
}

export function useProviders() {
    return useQuery<Provider[]>({
        queryKey: ['providers'],
        queryFn: () => apiClient.get<Provider[]>('/api/v1/providers'),
        staleTime: 1000 * 60 * 60, // Cache for 1 hour
        retry: 1,
    });
}
