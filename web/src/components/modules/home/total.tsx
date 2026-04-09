'use client';

import { motion } from 'motion/react';
import {
    CheckCircle,
    XCircle,
    Clock,
    ArrowDownToLine,
    ArrowUpFromLine,
    DollarSign,
    Zap,
    Database,
    TrendingDown,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useStatsRange } from '@/api/endpoints/stats';
import { AnimatedNumber } from '@/components/common/AnimatedNumber';
import { EASING } from '@/lib/animations/fluid-transitions';
import { useHomeViewStore, formatDateForAPI } from './store';

export function Total() {
    const t = useTranslations('home.total');
    const { dateRange } = useHomeViewStore();

    const startDate = formatDateForAPI(dateRange.start);
    const endDate = formatDateForAPI(dateRange.end);
    const { data: statsRange } = useStatsRange(startDate, endDate);

    return (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {/* 左侧：核心指标 */}
            <motion.section
                className="rounded-3xl bg-card border-card-border border p-6 text-card-foreground"
                initial={{ opacity: 0, y: 20, filter: 'blur(8px)' }}
                animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
                transition={{ duration: 0.5, ease: EASING.easeOutExpo, delay: 0 }}
            >
                <h3 className="font-semibold text-base mb-4 flex items-center gap-2">
                    <Zap className="w-5 h-5 text-chart-4" />
                    {t('coreMetrics')}
                </h3>
                <div className="grid grid-cols-2 gap-4">
                    {/* 成功率 */}
                    <div className="space-y-1">
                        <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            <CheckCircle className="w-3.5 h-3.5 text-chart-2" />
                            {t('successRate')}
                        </div>
                        <div className="text-2xl font-bold">
                            <AnimatedNumber value={statsRange?.success_rate} />
                        </div>
                        <div className="text-xs text-muted-foreground">
                            <span className="text-chart-2">{statsRange?.request_success.formatted.value}{statsRange?.request_success.formatted.unit}</span>
                            <span className="mx-1">/</span>
                            <span className="text-destructive">{statsRange?.request_failed.formatted.value}{statsRange?.request_failed.formatted.unit}</span>
                        </div>
                    </div>

                    {/* 平均响应时间 */}
                    <div className="space-y-1">
                        <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            <Clock className="w-3.5 h-3.5 text-chart-5" />
                            {t('avgResponseTime')}
                        </div>
                        <div className="text-2xl font-bold">
                            <AnimatedNumber value={statsRange?.avg_response_time.formatted.value} />
                            <span className="text-base font-normal text-muted-foreground">{statsRange?.avg_response_time.formatted.unit}</span>
                        </div>
                    </div>

                    {/* 请求次数 */}
                    <div className="space-y-1">
                        <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            <Database className="w-3.5 h-3.5 text-primary" />
                            {t('requestCount')}
                        </div>
                        <div className="text-2xl font-bold">
                            <AnimatedNumber value={statsRange?.request_count.formatted.value} />
                            <span className="text-base font-normal text-muted-foreground">{statsRange?.request_count.formatted.unit}</span>
                        </div>
                    </div>

                    {/* Token 用量 */}
                    <div className="space-y-1">
                        <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            <Zap className="w-3.5 h-3.5 text-chart-4" />
                            {t('totalToken')}
                        </div>
                        <div className="text-2xl font-bold">
                            <AnimatedNumber value={statsRange?.total_tokens.formatted.value} />
                            <span className="text-base font-normal text-muted-foreground">{statsRange?.total_tokens.formatted.unit}</span>
                        </div>
                        <div className="text-xs text-muted-foreground">
                            <ArrowDownToLine className="w-3 h-3 inline mr-1" />
                            {statsRange?.input_tokens.formatted.value}{statsRange?.input_tokens.formatted.unit}
                            <span className="mx-1">/</span>
                            <ArrowUpFromLine className="w-3 h-3 inline mr-1" />
                            {statsRange?.output_tokens.formatted.value}{statsRange?.output_tokens.formatted.unit}
                        </div>
                    </div>
                </div>
            </motion.section>

            {/* 右侧：成本分析 */}
            <motion.section
                className="rounded-3xl bg-card border-card-border border p-6 text-card-foreground"
                initial={{ opacity: 0, y: 20, filter: 'blur(8px)' }}
                animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
                transition={{ duration: 0.5, ease: EASING.easeOutExpo, delay: 0.08 }}
            >
                <h3 className="font-semibold text-base mb-4 flex items-center gap-2">
                    <DollarSign className="w-5 h-5 text-chart-1" />
                    {t('costAnalysis')}
                </h3>
                <div className="grid grid-cols-2 gap-4">
                    {/* 总成本 */}
                    <div className="space-y-1">
                        <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            <DollarSign className="w-3.5 h-3.5 text-chart-1" />
                            {t('totalCost')}
                        </div>
                        <div className="text-2xl font-bold text-chart-1">
                            <AnimatedNumber value={statsRange?.total_cost.formatted.value} />
                            <span className="text-base font-normal text-muted-foreground">{statsRange?.total_cost.formatted.unit}</span>
                        </div>
                        <div className="text-xs text-muted-foreground">
                            <ArrowDownToLine className="w-3 h-3 inline mr-1" />
                            {statsRange?.input_cost.formatted.value}{statsRange?.input_cost.formatted.unit}
                            <span className="mx-1">/</span>
                            <ArrowUpFromLine className="w-3 h-3 inline mr-1" />
                            {statsRange?.output_cost.formatted.value}{statsRange?.output_cost.formatted.unit}
                        </div>
                    </div>

                    {/* 缓存节省 */}
                    <div className="space-y-1">
                        <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            <TrendingDown className="w-3.5 h-3.5 text-chart-3" />
                            {t('costSaved')}
                        </div>
                        <div className="text-2xl font-bold text-chart-3">
                            <AnimatedNumber value={statsRange?.cost_saved.formatted.value} />
                            <span className="text-base font-normal text-muted-foreground">{statsRange?.cost_saved.formatted.unit}</span>
                        </div>
                    </div>

                    {/* 缓存效率 */}
                    <div className="space-y-1 col-span-2">
                        <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            <Database className="w-3.5 h-3.5 text-chart-3" />
                            {t('cacheEfficiency')}
                        </div>
                        <div className="grid grid-cols-2 gap-4 mt-2">
                            <div>
                                <div className="text-sm text-muted-foreground">{t('cacheHitRate')}</div>
                                <div className="text-xl font-semibold">{statsRange?.cache_hit_rate}</div>
                            </div>
                            <div>
                                <div className="text-sm text-muted-foreground">{t('cachedTokens')}</div>
                                <div className="text-xl font-semibold">
                                    {statsRange?.cached_tokens.formatted.value}
                                    <span className="text-sm font-normal text-muted-foreground">{statsRange?.cached_tokens.formatted.unit}</span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </motion.section>
        </div>
    );
}
