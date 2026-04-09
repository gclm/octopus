'use client';

import { motion } from 'motion/react';
import { Cpu, TrendingUp } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useStatsModels } from '@/api/endpoints/stats';
import { useHomeViewStore, formatDateForAPI } from './store';
import { EASING } from '@/lib/animations/fluid-transitions';

export function ModelDistribution() {
    const t = useTranslations('home');
    const { dateRange } = useHomeViewStore();

    const startDate = formatDateForAPI(dateRange.start);
    const endDate = formatDateForAPI(dateRange.end);
    const { data: modelStats } = useStatsModels(startDate, endDate);

    if (!modelStats || modelStats.length === 0) {
        return (
            <motion.div
                className="rounded-3xl bg-card border-card-border border p-6 text-card-foreground"
                initial={{ opacity: 0, y: 20, filter: 'blur(8px)' }}
                animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
                transition={{ duration: 0.5, ease: EASING.easeOutExpo, delay: 0.16 }}
            >
                <h3 className="font-semibold text-base mb-4 flex items-center gap-2">
                    <Cpu className="w-5 h-5 text-chart-4" />
                    {t('modelDistribution.title')}
                </h3>
                <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
                    <TrendingUp className="w-12 h-12 mb-3 opacity-30" />
                    <p className="text-sm">{t('modelDistribution.noData')}</p>
                </div>
            </motion.div>
        );
    }

    return (
        <motion.div
            className="rounded-3xl bg-card border-card-border border p-6 text-card-foreground"
            initial={{ opacity: 0, y: 20, filter: 'blur(8px)' }}
            animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
            transition={{ duration: 0.5, ease: EASING.easeOutExpo, delay: 0.16 }}
        >
            <h3 className="font-semibold text-base mb-4 flex items-center gap-2">
                <Cpu className="w-5 h-5 text-chart-4" />
                {t('modelDistribution.title')}
            </h3>
            <div className="space-y-3 max-h-[400px] overflow-y-auto">
                {modelStats.map((model, index) => (
                    <div
                        key={model.name}
                        className="flex items-center gap-3 p-3 rounded-2xl hover:bg-accent/5 transition-colors"
                    >
                        <div className="w-8 h-8 rounded-lg flex items-center justify-center font-bold text-sm shrink-0 bg-primary/10 text-primary">
                            {index + 1}
                        </div>

                        <div className="flex-1 min-w-0">
                            <p className="font-medium text-sm truncate">{model.name}</p>
                            <div className="flex items-center gap-2 mt-1">
                                <div className="flex-1 h-1.5 bg-muted rounded-full overflow-hidden">
                                    <div
                                        className="h-full bg-chart-4 rounded-full transition-all duration-500"
                                        style={{ width: model.percentage }}
                                    />
                                </div>
                                <span className="text-xs text-muted-foreground shrink-0">
                                    {model.percentage}
                                </span>
                            </div>
                        </div>

                        <div className="text-right shrink-0 space-y-0.5">
                            <div className="text-sm font-medium tabular-nums">
                                {model.request_count.formatted.value}
                                <span className="text-xs text-muted-foreground ml-0.5">
                                    {model.request_count.formatted.unit}
                                </span>
                            </div>
                            <div className="text-xs text-muted-foreground tabular-nums">
                                {model.total_cost.formatted.value}
                                {model.total_cost.formatted.unit}
                            </div>
                        </div>
                    </div>
                ))}
            </div>
        </motion.div>
    );
}
