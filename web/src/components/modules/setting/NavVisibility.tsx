'use client';

import { useTranslations } from 'next-intl';
import { Eye } from 'lucide-react';
import { Switch } from '@/components/ui/switch';
import { useSettingStore } from '@/stores/setting';
import { ROUTES } from '@/route/config';

export function SettingNavVisibility() {
    const t = useTranslations('setting');
    const tNav = useTranslations('navbar');
    const { hiddenNavItems, toggleNavItem } = useSettingStore();

    return (
        <div className="rounded-3xl border border-border bg-card p-6 space-y-5">
            <h2 className="text-lg font-bold text-card-foreground flex items-center gap-2">
                <Eye className="h-5 w-5" />
                {t('navVisibility.title')}
            </h2>
            <p className="text-sm text-muted-foreground">
                {t('navVisibility.description')}
            </p>
            <div className="space-y-3">
                {ROUTES.map((route) => {
                    const isHidden = hiddenNavItems.includes(route.id);
                    const isLocked = route.id === 'setting';
                    return (
                        <div key={route.id} className="flex items-center justify-between gap-4">
                            <div className="flex items-center gap-3">
                                <route.icon className="h-5 w-5 text-muted-foreground" />
                                <span className="text-sm font-medium">{tNav(route.id)}</span>
                                {isLocked && (
                                    <span className="text-xs text-muted-foreground">
                                        {t('navVisibility.locked')}
                                    </span>
                                )}
                            </div>
                            <Switch
                                checked={isLocked || !isHidden}
                                disabled={isLocked}
                                onCheckedChange={() => toggleNavItem(route.id)}
                            />
                        </div>
                    );
                })}
            </div>
        </div>
    );
}
