import { apiClient } from '../client';
import { useQuery } from '@tanstack/react-query';

export interface ProviderEndpoint {
    type: number;
    base_url: string;
}

export interface Provider {
    name: string;
    endpoints: ProviderEndpoint[];
}

export function useProviders() {
    return useQuery<Provider[]>({
        queryKey: ['providers'],
        queryFn: () => apiClient.get<Provider[]>('/api/v1/providers'),
        staleTime: 1000 * 60 * 60, // Cache for 1 hour
        retry: 1,
    });
}
