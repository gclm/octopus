'use client';

import { Activity } from './activity';
import { Total } from './total';
import { StatsChart } from './chart';
import { Rank } from './rank';
import { ModelDistribution } from './ModelDistribution';
import { DateRangePicker } from '@/components/common/DateRangePicker';
import { PageWrapper } from '@/components/common/PageWrapper';
import { motion } from 'motion/react';
import { EASING } from '@/lib/animations/fluid-transitions';
import { useHomeViewStore } from './store';
import { Tabs, TabsList, TabsTrigger, TabsContents, TabsContent } from '@/components/animate-ui/components/animate/tabs';
import { useTranslations } from 'next-intl';

export function Home() {
    const t = useTranslations('home');
    const activeView = useHomeViewStore((state) => state.activeView);
    const setActiveView = useHomeViewStore((state) => state.setActiveView);

    return (
        <PageWrapper className="h-full min-h-0 overflow-y-auto overscroll-contain space-y-4 pb-24 md:pb-4 rounded-t-3xl">
            {/* 时间选择器 */}
            <motion.div
                initial={{ opacity: 0, y: -10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.3, ease: EASING.easeOutExpo }}
            >
                <DateRangePicker />
            </motion.div>

            {/* 核心指标卡片 */}
            <Total />

            {/* Tab 切换：趋势 / 模型 / 渠道 */}
            <motion.div
                initial={{ opacity: 0, y: 20, filter: 'blur(8px)' }}
                animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
                transition={{ duration: 0.5, ease: EASING.easeOutExpo, delay: 0.12 }}
            >
                <Tabs value={activeView} onValueChange={(value) => setActiveView(value as 'trend' | 'model' | 'channel')}>
                    <TabsList className="mb-4">
                        <TabsTrigger value="trend">{t('tabs.trend')}</TabsTrigger>
                        <TabsTrigger value="model">{t('tabs.model')}</TabsTrigger>
                        <TabsTrigger value="channel">{t('tabs.channel')}</TabsTrigger>
                    </TabsList>
                    <TabsContents>
                        <TabsContent value="trend">
                            <div className="space-y-4">
                                <StatsChart />
                                <Activity />
                            </div>
                        </TabsContent>
                        <TabsContent value="model">
                            <ModelDistribution />
                        </TabsContent>
                        <TabsContent value="channel">
                            <Rank />
                        </TabsContent>
                    </TabsContents>
                </Tabs>
            </motion.div>
        </PageWrapper>
    );
}
