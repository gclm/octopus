'use client';

import { useState } from 'react';
import { useHomeViewStore, formatDateForAPI } from '@/components/modules/home/store';
import { CalendarIcon } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Calendar } from '@/components/ui/calendar';
import { useTranslations } from 'next-intl';
import type { DateRange } from 'react-day-picker';

const presets = [
    { key: 'today', label: 'today' },
    { key: '3days', label: '3days' },
    { key: '7days', label: '7days' },
    { key: '30days', label: '30days' },
];

export function DateRangePicker() {
    const t = useTranslations('home.dateRange');
    const { dateRangePreset, dateRange, setDateRangePreset, setDateRange } = useHomeViewStore();
    const [isOpen, setIsOpen] = useState(false);

    const handlePresetClick = (preset: typeof presets[number]) => {
        setDateRangePreset(preset.key as 'today' | '3days' | '7days' | '30days' | 'custom');
        setIsOpen(false);
    };

    const handleDateSelect = (range: DateRange | undefined) => {
        if (range?.from && range?.to) {
            setDateRange({ start: range.from, end: range.to });
            setIsOpen(false);
        }
    };

    const formatDisplayDate = (date: Date) => {
        return date.toLocaleDateString('zh-CN', {
            month: 'short',
            day: 'numeric',
        });
    };

    const getDisplayLabel = () => {
        if (dateRangePreset === 'custom') {
            return `${formatDisplayDate(dateRange.start)} - ${formatDisplayDate(dateRange.end)}`;
        }
        return t(presets.find(p => p.key === dateRangePreset)?.label || 'custom');
    };

    return (
        <Popover open={isOpen} onOpenChange={setIsOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    size="sm"
                    className="gap-2"
                >
                    <CalendarIcon className="w-4 h-4" />
                    {getDisplayLabel()}
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-auto p-0" align="start">
                <div className="p-3 space-y-3">
                    <div className="flex gap-2">
                        {presets.map((preset) => (
                            <Button
                                key={preset.key}
                                variant={dateRangePreset === preset.key ? 'default' : 'ghost'}
                                size="sm"
                                onClick={() => handlePresetClick(preset)}
                                className="flex-1"
                            >
                                {t(preset.label)}
                            </Button>
                        ))}
                    </div>
                    <div className="border-t pt-3">
                        <div className="text-xs text-muted-foreground mb-2">
                            {t('custom')}
                        </div>
                        <Calendar
                            mode="range"
                            selected={{ from: dateRange.start, to: dateRange.end }}
                            onSelect={handleDateSelect}
                            numberOfMonths={1}
                        />
                    </div>
                </div>
            </PopoverContent>
        </Popover>
    );
}

// Export helper for convenience
export { formatDateForAPI };
