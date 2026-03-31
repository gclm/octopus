'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import type { LucideIcon } from 'lucide-react';
import { Compass, Gauge, Hash, HelpCircle, Timer, TimerOff, Waves, Zap } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { useSettingList, useSetSetting, SettingKey } from '@/api/endpoints/setting';
import { toast } from '@/components/common/Toast';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/animate-ui/components/animate/tooltip';

type FieldConfig = {
    field: string;
    labelKey: string;
    placeholderKey: string;
    icon: LucideIcon;
};

const basicFields: FieldConfig[] = [
    {
        field: SettingKey.CircuitBreakerThreshold,
        labelKey: 'threshold.label',
        placeholderKey: 'threshold.placeholder',
        icon: Hash,
    },
    {
        field: SettingKey.CircuitBreakerCooldown,
        labelKey: 'cooldown.label',
        placeholderKey: 'cooldown.placeholder',
        icon: Timer,
    },
    {
        field: SettingKey.CircuitBreakerMaxCooldown,
        labelKey: 'maxCooldown.label',
        placeholderKey: 'maxCooldown.placeholder',
        icon: TimerOff,
    },
];

const healthFields: FieldConfig[] = [
    {
        field: SettingKey.CircuitBreakerHealthScoreThreshold,
        labelKey: 'healthScoreThreshold.label',
        placeholderKey: 'healthScoreThreshold.placeholder',
        icon: Gauge,
    },
    {
        field: SettingKey.CircuitBreakerHealthScoreMin,
        labelKey: 'healthScoreMin.label',
        placeholderKey: 'healthScoreMin.placeholder',
        icon: Gauge,
    },
    {
        field: SettingKey.CircuitBreakerHealthScoreMax,
        labelKey: 'healthScoreMax.label',
        placeholderKey: 'healthScoreMax.placeholder',
        icon: Gauge,
    },
    {
        field: SettingKey.CircuitBreakerHealthScoreDecayStep,
        labelKey: 'healthScoreDecayStep.label',
        placeholderKey: 'healthScoreDecayStep.placeholder',
        icon: Waves,
    },
    {
        field: SettingKey.CircuitBreakerHealthScoreDecayIntervalSeconds,
        labelKey: 'healthScoreDecayIntervalSeconds.label',
        placeholderKey: 'healthScoreDecayIntervalSeconds.placeholder',
        icon: Timer,
    },
    {
        field: SettingKey.CircuitBreakerHealthScoreWarmupSuccesses,
        labelKey: 'healthScoreWarmupSuccesses.label',
        placeholderKey: 'healthScoreWarmupSuccesses.placeholder',
        icon: Zap,
    },
    {
        field: SettingKey.CircuitBreakerExplorationEvery,
        labelKey: 'explorationEvery.label',
        placeholderKey: 'explorationEvery.placeholder',
        icon: Compass,
    },
];

const allFields = [...basicFields, ...healthFields];

function FieldGrid({
    fields,
    values,
    onChange,
    onBlur,
    t,
}: {
    fields: FieldConfig[];
    values: Record<string, string>;
    onChange: (field: string, value: string) => void;
    onBlur: (field: string) => void;
    t: ReturnType<typeof useTranslations<'setting'>>;
}) {
    return (
        <div className="grid gap-4 md:grid-cols-2">
            {fields.map(({ field, labelKey, placeholderKey, icon: Icon }) => (
                <label key={field} className="rounded-2xl border border-border/60 bg-background/70 p-4 space-y-3">
                    <div className="flex items-center gap-3">
                        <div className="flex size-9 items-center justify-center rounded-xl bg-muted text-muted-foreground">
                            <Icon className="h-4 w-4" />
                        </div>
                        <span className="text-sm font-medium leading-5">{t(`circuitBreaker.${labelKey}`)}</span>
                    </div>
                    <Input
                        type="number"
                        value={values[field] ?? ''}
                        onChange={(e) => onChange(field, e.target.value)}
                        onBlur={() => onBlur(field)}
                        placeholder={t(`circuitBreaker.${placeholderKey}`)}
                        className="rounded-xl"
                    />
                </label>
            ))}
        </div>
    );
}

export function SettingCircuitBreaker() {
    const t = useTranslations('setting');
    const { data: settings } = useSettingList();
    const setSetting = useSetSetting();

    const [values, setValues] = useState<Record<string, string>>({});
    const initialValues = useRef<Record<string, string>>({});
    const fields = useMemo(() => allFields, []);

    useEffect(() => {
        if (!settings) return;

        const nextValues: Record<string, string> = {};
        for (const field of fields) {
            const setting = settings.find((item) => item.key === field.field);
            nextValues[field.field] = setting?.value ?? '';
        }
        initialValues.current = nextValues;
        queueMicrotask(() => setValues(nextValues));
    }, [fields, settings]);

    const handleChange = (field: string, value: string) => {
        setValues((prev) => ({ ...prev, [field]: value }));
    };

    const handleSave = (field: string) => {
        const value = values[field] ?? '';
        const initialValue = initialValues.current[field] ?? '';
        if (value === initialValue) return;

        setSetting.mutate(
            { key: field, value },
            {
                onSuccess: () => {
                    toast.success(t('saved'));
                    initialValues.current = { ...initialValues.current, [field]: value };
                },
            },
        );
    };

    return (
        <div className="rounded-3xl border border-border bg-card p-6 space-y-5">
            <div className="flex items-center gap-2 text-lg font-bold text-card-foreground">
                <Zap className="h-5 w-5" />
                <span>{t('circuitBreaker.title')}</span>
                <TooltipProvider>
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <HelpCircle className="size-4 cursor-help text-muted-foreground" />
                        </TooltipTrigger>
                        <TooltipContent>{t('circuitBreaker.hint')}</TooltipContent>
                    </Tooltip>
                </TooltipProvider>
            </div>

            <div className="space-y-3">
                <div className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                    {t('circuitBreaker.sections.basic')}
                </div>
                <FieldGrid fields={basicFields} values={values} onChange={handleChange} onBlur={handleSave} t={t} />
            </div>

            <div className="space-y-3">
                <div className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                    {t('circuitBreaker.sections.health')}
                </div>
                <FieldGrid fields={healthFields} values={values} onChange={handleChange} onBlur={handleSave} t={t} />
            </div>
        </div>
    );
}
