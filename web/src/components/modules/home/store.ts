'use client';

import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

export type RankSortMode = 'cost' | 'count' | 'tokens';
export type ChartMetricType = 'cost' | 'count' | 'tokens';
export type ChartPeriod = '1' | '7' | '30';
export type DateRangePreset = 'today' | '3days' | '7days' | '30days' | 'custom';
export type ActiveView = 'trend' | 'model' | 'channel';

interface HomeViewState {
    rankSortMode: RankSortMode;
    chartMetricType: ChartMetricType;
    chartPeriod: ChartPeriod;
    dateRangePreset: DateRangePreset;
    dateRange: { start: Date; end: Date };
    activeView: ActiveView;

    setRankSortMode: (value: RankSortMode) => void;
    setChartMetricType: (value: ChartMetricType) => void;
    setChartPeriod: (value: ChartPeriod) => void;
    setDateRangePreset: (value: DateRangePreset) => void;
    setDateRange: (range: { start: Date; end: Date }) => void;
    setActiveView: (view: ActiveView) => void;
}

// Helper: generate date range from preset
function getDateRangeFromPreset(preset: DateRangePreset): { start: Date; end: Date } {
    const end = new Date();
    end.setHours(23, 59, 59, 999);
    const start = new Date(end);

    switch (preset) {
        case 'today':
            start.setHours(0, 0, 0, 0);
            break;
        case '3days':
            start.setDate(start.getDate() - 2);
            start.setHours(0, 0, 0, 0);
            break;
        case '7days':
            start.setDate(start.getDate() - 6);
            start.setHours(0, 0, 0, 0);
            break;
        case '30days':
            start.setDate(start.getDate() - 29);
            start.setHours(0, 0, 0, 0);
            break;
        case 'custom':
            return { start: end, end };
    }

    return { start, end };
}

// Helper: format date for API (YYYYMMDD)
export function formatDateForAPI(date: Date): string {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}${month}${day}`;
}

export { getDateRangeFromPreset };

export const useHomeViewStore = create<HomeViewState>()(
    persist(
        (set) => ({
            rankSortMode: 'cost',
            chartMetricType: 'cost',
            chartPeriod: '1',
            dateRangePreset: 'today',
            dateRange: getDateRangeFromPreset('today'),
            activeView: 'trend',
            setRankSortMode: (value) => set({ rankSortMode: value }),
            setChartMetricType: (value) => set({ chartMetricType: value }),
            setChartPeriod: (value) => set({ chartPeriod: value }),
            setDateRangePreset: (value) => set({
                dateRangePreset: value,
                dateRange: getDateRangeFromPreset(value),
            }),
            setDateRange: (range) => set({
                dateRangePreset: 'custom',
                dateRange: range,
            }),
            setActiveView: (view) => set({ activeView: view }),
        }),
        {
            name: 'home-view-options-storage',
            storage: createJSONStorage(() => localStorage),
            partialize: (state) => ({
                rankSortMode: state.rankSortMode,
                chartMetricType: state.chartMetricType,
                chartPeriod: state.chartPeriod,
                dateRangePreset: state.dateRangePreset,
                activeView: state.activeView,
            }),
        }
    )
);
