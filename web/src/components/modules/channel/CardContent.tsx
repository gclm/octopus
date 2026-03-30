import { useState } from 'react';
import {
    Trash2,
    CheckCircle2,
    XCircle,
    FileText,
    DollarSign,
    Clock,
    Activity,
    TrendingUp,
    Globe,
    Key,
    Gauge,
    Snowflake,
    TimerReset,
    ShieldAlert,
} from 'lucide-react';
import { useUpdateChannel, useDeleteChannel, type Channel, type UpdateChannelRequest } from '@/api/endpoints/channel';
import {
    MorphingDialogTitle,
    MorphingDialogDescription,
    MorphingDialogClose,
    useMorphingDialog,
} from '@/components/ui/morphing-dialog';
import { Tabs, TabsContents, TabsContent } from '@/components/animate-ui/primitives/animate/tabs';
import { type StatsMetricsFormatted } from '@/api/endpoints/stats';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { ChannelForm, type ChannelFormData } from './Form';
import { formatMoney } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { formatCooldown, formatSignedScore, getDisplaySummaryScore, getHealthTone, getRouteDisplayScore, healthBadgeClassName } from './health';

export function CardContent({ channel, stats }: { channel: Channel; stats: StatsMetricsFormatted }) {
    const { setIsOpen } = useMorphingDialog();
    const updateChannel = useUpdateChannel();
    const deleteChannel = useDeleteChannel();
    const [isEditing, setIsEditing] = useState(false);
    const [isConfirmingDelete, setIsConfirmingDelete] = useState(false);
    const [formData, setFormData] = useState<ChannelFormData>({
        name: channel.name,
        type: channel.type,
        enabled: channel.enabled,
        base_urls: channel.base_urls?.length ? channel.base_urls : [{ url: '', delay: 0 }],
        custom_header: channel.custom_header ?? [],
        channel_proxy: channel.channel_proxy ?? '',
        param_override: channel.param_override ?? '',
        keys: channel.keys.length > 0
            ? channel.keys.map((k) => ({
                id: k.id,
                enabled: k.enabled,
                channel_key: k.channel_key,
                status_code: k.status_code,
                last_use_time_stamp: k.last_use_time_stamp,
                total_cost: k.total_cost,
                remark: k.remark,
            }))
            : [{ enabled: true, channel_key: '', remark: '' }],
        model: channel.model,
        custom_model: channel.custom_model,
        proxy: channel.proxy,
        auto_sync: channel.auto_sync,
        auto_group: channel.auto_group,
        match_regex: channel.match_regex ?? '',
    });
    const t = useTranslations('channel.detail');
    const tHealth = useTranslations('channel.health');
    const health = channel.health_summary;
    const healthScore = getDisplaySummaryScore(health);

    const currentView = isEditing ? 'editing' : 'viewing';

    const baseUrlsEqual = (a: Channel['base_urls'] | undefined, b: Channel['base_urls'] | undefined) =>
        JSON.stringify(a ?? []) === JSON.stringify(b ?? []);
    const headersEqual = (a: Channel['custom_header'] | undefined, b: Channel['custom_header'] | undefined) =>
        JSON.stringify(a ?? []) === JSON.stringify(b ?? []);

    const handleUpdate = (event: React.FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        const req: UpdateChannelRequest = { id: channel.id };

        // only send changed fields to avoid accidental clears
        if (formData.name !== channel.name) req.name = formData.name;
        if (formData.type !== channel.type) req.type = formData.type;
        if (formData.enabled !== channel.enabled) req.enabled = formData.enabled;
        if (!baseUrlsEqual(formData.base_urls, channel.base_urls)) {
            req.base_urls = (formData.base_urls ?? []).filter((u) => u.url.trim()).map((u) => ({
                url: u.url.trim(),
                delay: Number(u.delay || 0),
            }));
        }
        if (formData.model !== channel.model) req.model = formData.model;
        if (formData.custom_model !== channel.custom_model) req.custom_model = formData.custom_model;
        if (formData.proxy !== channel.proxy) req.proxy = formData.proxy;
        if (formData.auto_sync !== channel.auto_sync) req.auto_sync = formData.auto_sync;
        if (formData.auto_group !== channel.auto_group) req.auto_group = formData.auto_group;

        if (!headersEqual(formData.custom_header, channel.custom_header)) {
            req.custom_header = (formData.custom_header ?? [])
                .map((h) => ({ header_key: h.header_key.trim(), header_value: h.header_value }))
                .filter((h) => h.header_key && h.header_value !== '');
        }

        const nextChannelProxy = formData.channel_proxy.trim();
        const curChannelProxy = channel.channel_proxy ?? '';
        if (nextChannelProxy !== curChannelProxy) {
            // Empty string means "clear" for patch semantics; backend maps it to NULL.
            req.channel_proxy = nextChannelProxy;
        }

        const nextParamOverride = formData.param_override.trim();
        const curParamOverride = channel.param_override ?? '';
        if (nextParamOverride !== curParamOverride) {
            // Empty string means "clear" for patch semantics; backend maps it to NULL.
            req.param_override = nextParamOverride;
        }

        const nextMatchRegex = formData.match_regex.trim();
        const curMatchRegex = channel.match_regex ?? '';
        if (nextMatchRegex !== curMatchRegex) {
            // Empty string means "clear" for patch semantics; backend maps it to NULL.
            req.match_regex = nextMatchRegex;
        }

        const originalKeys = channel.keys;
        const originalByID = new Map(originalKeys.map((k) => [k.id, k]));
        const nextKeys = formData.keys ?? [];

        const nextIDs = new Set(nextKeys.filter((k) => typeof k.id === 'number').map((k) => k.id as number));
        const keys_to_delete = originalKeys.filter((k) => !nextIDs.has(k.id)).map((k) => k.id);

        const keys_to_add = nextKeys
            .filter((k) => !k.id && k.channel_key.trim())
            .map((k) => ({ enabled: k.enabled, channel_key: k.channel_key, remark: k.remark ?? '' }));

        const keys_to_update = nextKeys
            .filter((k) => typeof k.id === 'number' && originalByID.has(k.id as number))
            .map((k) => {
                const orig = originalByID.get(k.id as number)!;
                const u: { id: number; enabled?: boolean; channel_key?: string; remark?: string } = { id: k.id as number };
                if (k.enabled !== orig.enabled) u.enabled = k.enabled;
                if (k.channel_key !== orig.channel_key) u.channel_key = k.channel_key;
                if ((k.remark ?? '') !== orig.remark) u.remark = k.remark ?? '';
                return Object.keys(u).length > 1 ? u : null;
            })
            .filter((u) => u !== null) as Array<{ id: number; enabled?: boolean; channel_key?: string; remark?: string }>;

        if (keys_to_add.length > 0) req.keys_to_add = keys_to_add;
        if (keys_to_update.length > 0) req.keys_to_update = keys_to_update;
        if (keys_to_delete.length > 0) req.keys_to_delete = keys_to_delete;

        updateChannel.mutate(req, {
            onSuccess: () => {
                setIsEditing(false);
                setIsOpen(false);
            }
        });
    };

    const handleDeleteClick = () => {
        if (!isConfirmingDelete) {
            setIsConfirmingDelete(true);
            return;
        }

        setIsOpen(false);
        setTimeout(() => {
            deleteChannel.mutate(channel.id);
        }, 300);
    };

    return (
        <>
            <MorphingDialogTitle>
                <header className="mb-6 flex items-center justify-between">
                    <h2 className="text-2xl font-bold text-card-foreground">
                        {isEditing ? t('title.edit') : t('title.view')}
                    </h2>
                    <MorphingDialogClose
                        className="relative top-0 right-0"
                        variants={{
                            initial: { opacity: 0, scale: 0.8 },
                            animate: { opacity: 1, scale: 1 },
                            exit: { opacity: 0, scale: 0.8 }
                        }}
                    />
                </header>
            </MorphingDialogTitle>

            <MorphingDialogDescription>
                <Tabs value={currentView}>
                    <TabsContents>
                        <TabsContent value="viewing" >
                            <div className="max-h-[60vh] overflow-y-auto space-y-4 sm:space-y-5">
                                <dl className="grid gap-3 grid-cols-1 sm:grid-cols-3">
                                    <div className="rounded-2xl border bg-linear-to-br from-chart-1/10 to-chart-1/5 p-3 sm:p-4">
                                        <dt className="flex items-center gap-2 mb-2 text-xs font-medium text-muted-foreground">
                                            <Activity className="size-4 text-chart-1" />
                                            {t('metrics.totalRequests')}
                                        </dt>
                                        <dd className="text-xl sm:text-2xl font-bold text-chart-1">
                                            {stats.request_count.formatted.value}
                                            <span className="text-xs font-normal ml-1 text-muted-foreground">{stats.request_count.formatted.unit}</span>
                                        </dd>
                                    </div>

                                    <div className="rounded-2xl border bg-linear-to-br from-chart-3/10 to-chart-3/5 p-3 sm:p-4">
                                        <dt className="flex items-center gap-2 mb-2 text-xs font-medium text-muted-foreground">
                                            <FileText className="size-4 text-chart-3" />
                                            {t('metrics.totalToken')}
                                        </dt>
                                        <dd className="text-xl sm:text-2xl font-bold text-chart-3">
                                            {stats.total_token.formatted.value}
                                            <span className="text-xs font-normal ml-1 text-muted-foreground">{stats.total_token.formatted.unit}</span>
                                        </dd>
                                    </div>

                                    <div className="rounded-2xl border bg-linear-to-br from-chart-5/10 to-chart-5/5 p-3 sm:p-4">
                                        <dt className="flex items-center gap-2 mb-2 text-xs font-medium text-muted-foreground">
                                            <DollarSign className="size-4 text-chart-5" />
                                            {t('metrics.totalCost')}
                                        </dt>
                                        <dd className="text-xl sm:text-2xl font-bold text-chart-5">
                                            {stats.total_cost.formatted.value}
                                            <span className="text-xs font-normal ml-1 text-muted-foreground">{stats.total_cost.formatted.unit}</span>
                                        </dd>
                                    </div>
                                </dl>

                                <section className="space-y-3">
                                    <h4 className="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                                        <Gauge className="size-3.5" />
                                        {t('sections.health')}
                                    </h4>
                                    <dl className="grid gap-3 grid-cols-1 sm:grid-cols-2 xl:grid-cols-4">
                                        <div className="rounded-2xl border bg-card p-3 sm:p-4 transition-colors hover:bg-accent/5">
                                            <dt className="mb-2 flex items-center gap-2 text-xs text-muted-foreground">
                                                <ShieldAlert className="size-4" />
                                                {tHealth('labels.status')}
                                            </dt>
                                            <dd>
                                                <Badge variant="secondary" className={cn('text-xs', getHealthTone(health?.status))}>
                                                    {tHealth(`status.${health?.status ?? 'idle'}`)}
                                                </Badge>
                                            </dd>
                                        </div>

                                        <div className="rounded-2xl border bg-card p-3 sm:p-4 transition-colors hover:bg-accent/5">
                                            <dt className="mb-2 flex items-center gap-2 text-xs text-muted-foreground">
                                                <Gauge className="size-4 text-primary" />
                                                {tHealth('labels.routingScore')}
                                            </dt>
                                            <dd className="text-2xl font-bold text-card-foreground">
                                                {formatSignedScore(healthScore)}
                                            </dd>
                                        </div>

                                        <div className="rounded-2xl border bg-card p-3 sm:p-4 transition-colors hover:bg-accent/5">
                                            <dt className="mb-2 flex items-center gap-2 text-xs text-muted-foreground">
                                                <Snowflake className="size-4 text-destructive" />
                                                {tHealth('labels.coolingRoutes')}
                                            </dt>
                                            <dd className="text-2xl font-bold text-card-foreground">
                                                {health?.cooling_routes ?? 0}
                                            </dd>
                                        </div>

                                        <div className="rounded-2xl border bg-card p-3 sm:p-4 transition-colors hover:bg-accent/5">
                                            <dt className="mb-2 flex items-center gap-2 text-xs text-muted-foreground">
                                                <TimerReset className="size-4 text-sky-500" />
                                                {tHealth('labels.warmupRoutes')}
                                            </dt>
                                            <dd className="text-2xl font-bold text-card-foreground">
                                                {health?.warmup_routes ?? 0}
                                            </dd>
                                        </div>
                                    </dl>

                                    <div className="rounded-2xl border bg-card p-3 sm:p-4">
                                        {health && health.tracked_routes > 0 ? (
                                            <div className="flex flex-wrap gap-2">
                                                <Badge variant="secondary" className="h-6 px-2 text-xs">
                                                    {tHealth('labels.trackedRoutes')}: {health.tracked_routes}
                                                </Badge>
                                                <Badge variant="secondary" className="h-6 px-2 text-xs">
                                                    {tHealth('labels.trackedKeys')}: {health.tracked_keys}
                                                </Badge>
                                                <Badge variant="secondary" className="h-6 px-2 text-xs">
                                                    {tHealth('labels.worstRawScore')}: {formatSignedScore(health.worst_raw_score)}
                                                </Badge>
                                                {health.cooldown_remaining_ms > 0 && (
                                                    <Badge variant="secondary" className="h-6 px-2 text-xs bg-red-500/10 text-red-700 dark:text-red-400">
                                                        {tHealth('labels.cooldown')}: {formatCooldown(health.cooldown_remaining_ms)}
                                                    </Badge>
                                                )}
                                                {health.last_failure_kind && health.last_failure_kind !== 'unknown' && (
                                                    <Badge variant="secondary" className="h-6 px-2 text-xs">
                                                        {tHealth(`failures.${health.last_failure_kind}` as never)}
                                                    </Badge>
                                                )}
                                            </div>
                                        ) : (
                                            <div className="text-sm text-muted-foreground">{tHealth('noSignals')}</div>
                                        )}
                                    </div>
                                </section>

                                {/* 请求详情 */}
                                <section className="space-y-3">
                                    <h4 className="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                                        <TrendingUp className="size-3.5" />
                                        {t('sections.requests')}
                                    </h4>
                                    <dl className="grid gap-3 grid-cols-1 sm:grid-cols-2">
                                        <div className="rounded-2xl border bg-card p-3 sm:p-4 transition-colors hover:bg-accent/5">
                                            <dt className="flex items-center gap-2 mb-2 text-xs text-muted-foreground">
                                                <CheckCircle2 className="size-4 text-accent" />
                                                {t('metrics.successRequests')}
                                            </dt>
                                            <dd className="text-2xl font-bold text-accent">
                                                {stats.request_success.formatted.value}
                                                <span className="text-sm font-normal ml-1 text-muted-foreground">{stats.request_success.formatted.unit}</span>
                                            </dd>
                                        </div>

                                        <div className="rounded-2xl border bg-card p-3 sm:p-4 transition-colors hover:bg-accent/5">
                                            <dt className="flex items-center gap-2 mb-2 text-xs text-muted-foreground">
                                                <XCircle className="size-4 text-destructive" />
                                                {t('metrics.failedRequests')}
                                            </dt>
                                            <dd className="text-2xl font-bold text-destructive">
                                                {stats.request_failed.formatted.value}
                                                <span className="text-sm font-normal ml-1 text-muted-foreground">{stats.request_failed.formatted.unit}</span>
                                            </dd>
                                        </div>
                                    </dl>
                                </section>

                                {/* Token 使用 */}
                                <section className="space-y-3">
                                    <h4 className="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                                        <FileText className="size-3.5" />
                                        {t('sections.tokens')}
                                    </h4>
                                    <dl className="grid gap-3 grid-cols-1 sm:grid-cols-2">
                                        <div className="rounded-2xl border bg-card p-3 sm:p-4 transition-colors hover:bg-accent/5">
                                            <dt className="flex items-center gap-2 mb-2 text-xs text-muted-foreground">
                                                <div className="size-2 rounded-full bg-chart-1" />
                                                {t('metrics.inputToken')}
                                            </dt>
                                            <dd className="text-2xl font-bold text-card-foreground">
                                                {stats.input_token.formatted.value}
                                                <span className="text-sm font-normal ml-1 text-muted-foreground">{stats.input_token.formatted.unit}</span>
                                            </dd>
                                        </div>

                                        <div className="rounded-2xl border bg-card p-3 sm:p-4 transition-colors hover:bg-accent/5">
                                            <dt className="flex items-center gap-2 mb-2 text-xs text-muted-foreground">
                                                <div className="size-2 rounded-full bg-chart-3" />
                                                {t('metrics.outputToken')}
                                            </dt>
                                            <dd className="text-2xl font-bold text-card-foreground">
                                                {stats.output_token.formatted.value}
                                                <span className="text-sm font-normal ml-1 text-muted-foreground">{stats.output_token.formatted.unit}</span>
                                            </dd>
                                        </div>
                                    </dl>
                                </section>

                                {/* 成本详情 */}
                                <section className="space-y-3">
                                    <h4 className="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                                        <DollarSign className="size-3.5" />
                                        {t('sections.costs')}
                                    </h4>
                                    <dl className="grid gap-3 grid-cols-1 sm:grid-cols-2">
                                        <div className="rounded-2xl border bg-card p-3 sm:p-4 transition-colors hover:bg-accent/5">
                                            <dt className="flex items-center gap-2 mb-2 text-xs text-muted-foreground">
                                                <div className="size-2 rounded-full bg-chart-2" />
                                                {t('metrics.inputCost')}
                                            </dt>
                                            <dd className="text-2xl font-bold text-card-foreground">
                                                {stats.input_cost.formatted.value}
                                                <span className="text-sm font-normal ml-1 text-muted-foreground">{stats.input_cost.formatted.unit}</span>
                                            </dd>
                                        </div>

                                        <div className="rounded-2xl border bg-card p-3 sm:p-4 transition-colors hover:bg-accent/5">
                                            <dt className="flex items-center gap-2 mb-2 text-xs text-muted-foreground">
                                                <div className="size-2 rounded-full bg-chart-5" />
                                                {t('metrics.outputCost')}
                                            </dt>
                                            <dd className="text-2xl font-bold text-card-foreground">
                                                {stats.output_cost.formatted.value}
                                                <span className="text-sm font-normal ml-1 text-muted-foreground">{stats.output_cost.formatted.unit}</span>
                                            </dd>
                                        </div>
                                    </dl>
                                </section>

                                {/* Base URLs */}
                                <section className="space-y-3">
                                    <h4 className="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                                        <Globe className="size-3.5" />
                                        {t('sections.baseUrls')}
                                    </h4>
                                    <div className="rounded-2xl border bg-card overflow-hidden">
                                        {channel.base_urls?.map((url, i) => (
                                            <div key={i} className="flex items-center justify-between p-3 sm:p-4 border-b last:border-0 hover:bg-accent/5 transition-colors">
                                                <div className="flex flex-col gap-1 min-w-0">
                                                    <span className="font-mono text-sm truncate select-all">{url.url}</span>
                                                </div>
                                                <Badge
                                                    variant="secondary"
                                                    className={cn(
                                                        "h-5 px-1.5 text-xs",
                                                        url.delay < 300
                                                            ? "bg-green-500/15 text-green-700 dark:text-green-400"
                                                            : url.delay < 1000
                                                                ? "bg-orange-500/15 text-orange-700 dark:text-orange-400"
                                                                : "bg-red-500/15 text-red-700 dark:text-red-400"
                                                    )}
                                                >
                                                    {url.delay}ms
                                                </Badge>
                                            </div>
                                        ))}
                                        {(!channel.base_urls || channel.base_urls.length === 0) && (
                                            <div className="p-4 text-sm text-muted-foreground text-center">{t('noBaseUrls')}</div>
                                        )}
                                    </div>
                                </section>

                                {/* Keys */}
                                <section className="space-y-3">
                                    <h4 className="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                                        <Key className="size-3.5" />
                                        {t('sections.keys')}
                                    </h4>
                                    <div className="rounded-2xl border bg-card overflow-hidden">
                                        {channel.keys?.map((key) => (
                                            <div key={key.id} className="border-b last:border-0 p-3 sm:p-4 transition-colors hover:bg-accent/5">
                                                <div className="flex items-center gap-3">
                                                    <div className={cn("size-2 shrink-0 rounded-full", key.enabled ? "bg-emerald-500" : "bg-destructive")} />

                                                    <span className="font-mono text-sm truncate min-w-0 flex-1">
                                                        {key.channel_key.length > 10
                                                            ? `${key.channel_key.slice(0, 4)}...${key.channel_key.slice(-4)}`
                                                            : key.channel_key}
                                                    </span>

                                                    {key.remark && (
                                                        <span className="text-xs text-muted-foreground truncate max-w-24" title={key.remark}>
                                                            {key.remark}
                                                        </span>
                                                    )}

                                                    <div className="flex items-center gap-2 shrink-0">
                                                        {key.last_use_time_stamp > 0 && (
                                                            <span className="text-xs text-muted-foreground whitespace-nowrap hidden sm:inline-block">
                                                                {new Date(key.last_use_time_stamp * 1000).toLocaleString()}
                                                            </span>
                                                        )}

                                                        {key.status_code !== 0 && (
                                                            <Badge
                                                                variant="secondary"
                                                                className={cn(
                                                                    "h-5 px-1.5 text-[10px]",
                                                                    key.status_code === 200
                                                                        ? "bg-green-500/15 text-green-700 dark:text-green-400"
                                                                        : key.status_code === 401 ||
                                                                            key.status_code === 403 ||
                                                                            key.status_code === 429 ||
                                                                            key.status_code >= 500
                                                                            ? "bg-red-500/15 text-red-700 dark:text-red-400"
                                                                            : "bg-orange-500/15 text-orange-700 dark:text-orange-400"
                                                                )}
                                                            >
                                                                {key.status_code}
                                                            </Badge>
                                                        )}

                                                        <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">
                                                            {formatMoney(key.total_cost).formatted.value}
                                                            {formatMoney(key.total_cost).formatted.unit}
                                                        </Badge>
                                                    </div>
                                                </div>

                                                {!!key.health_summary && key.health_summary.tracked_routes > 0 && (
                                                    <div className="mt-3 space-y-2">
                                                        <div className="flex flex-wrap gap-2">
                                                            <Badge variant="secondary" className={healthBadgeClassName(key.health_summary.status)}>
                                                                {tHealth(`status.${key.health_summary.status}` as never)}
                                                            </Badge>
                                                            <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">
                                                                {tHealth('labels.routingScore')}: {formatSignedScore(getDisplaySummaryScore(key.health_summary))}
                                                            </Badge>
                                                            {key.health_summary.cooldown_remaining_ms > 0 && (
                                                                <Badge variant="secondary" className="h-5 px-1.5 text-[10px] bg-red-500/10 text-red-700 dark:text-red-400">
                                                                    {tHealth('labels.cooldown')}: {formatCooldown(key.health_summary.cooldown_remaining_ms)}
                                                                </Badge>
                                                            )}
                                                            {key.health_summary.last_failure_kind && key.health_summary.last_failure_kind !== 'unknown' && (
                                                                <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">
                                                                    {tHealth(`failures.${key.health_summary.last_failure_kind}` as never)}
                                                                </Badge>
                                                            )}
                                                        </div>

                                                        {!!key.health_routes?.length && (
                                                            <div className="flex flex-wrap gap-2">
                                                                {key.health_routes.slice(0, 4).map((route) => (
                                                                    <div key={`${key.id}-${route.model_name}`} className="rounded-xl border border-border/60 bg-background/80 px-2.5 py-1.5 text-xs">
                                                                        <div className="font-medium text-foreground">{route.model_name}</div>
                                                                        <div className="mt-1 flex flex-wrap items-center gap-1.5 text-muted-foreground">
                                                                            <Badge variant="secondary" className={healthBadgeClassName(route.state === 'open' ? 'cooling' : route.warmup_pending ? 'warming' : route.raw_score < 0 ? 'degraded' : 'healthy')}>
                                                                                {tHealth(`status.${route.state === 'open' ? 'cooling' : route.warmup_pending ? 'warming' : route.raw_score < 0 ? 'degraded' : route.ordering_score > 0 ? 'healthy' : 'neutral'}` as never)}
                                                                            </Badge>
                                                                            <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">
                                                                                {tHealth('labels.scoreShort')}: {formatSignedScore(getRouteDisplayScore(route))}
                                                                            </Badge>
                                                                            {route.cooldown_remaining_ms > 0 && (
                                                                                <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">
                                                                                    {formatCooldown(route.cooldown_remaining_ms)}
                                                                                </Badge>
                                                                            )}
                                                                            {route.last_failure_kind !== 'unknown' && (
                                                                                <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">
                                                                                    {tHealth(`failures.${route.last_failure_kind}` as never)}
                                                                                </Badge>
                                                                            )}
                                                                        </div>
                                                                    </div>
                                                                ))}
                                                                {(key.health_routes?.length ?? 0) > 4 && (
                                                                    <Badge variant="secondary" className="h-6 px-2 text-xs">
                                                                        {tHealth('moreRoutes', { count: (key.health_routes?.length ?? 0) - 4 })}
                                                                    </Badge>
                                                                )}
                                                            </div>
                                                        )}
                                                    </div>
                                                )}
                                            </div>
                                        ))}
                                        {(!channel.keys || channel.keys.length === 0) && (
                                            <div className="p-4 text-sm text-muted-foreground text-center">{t('noKeys')}</div>
                                        )}
                                    </div>
                                </section>

                                {/* 等待时间 */}
                                <dl className="rounded-2xl border bg-card p-3 sm:p-4 transition-colors hover:bg-accent/5">
                                    <dt className="flex items-center gap-2 mb-2 text-xs text-muted-foreground">
                                        <Clock className="size-4 text-primary" />
                                        {t('metrics.avgWaitTime')}
                                    </dt>
                                    <dd className="text-2xl font-bold text-primary">
                                        {stats.wait_time.formatted.value}
                                        <span className="text-sm font-normal ml-1 text-muted-foreground">{stats.wait_time.formatted.unit}</span>
                                    </dd>
                                </dl>
                            </div>

                            {/* 操作按钮 */}
                            <div className="grid gap-3 sm:grid-cols-2 pt-2">
                                <Button
                                    onClick={() => (isConfirmingDelete ? setIsConfirmingDelete(false) : setIsEditing(true))}
                                    variant={isConfirmingDelete ? 'secondary' : 'default'}
                                    className="w-full rounded-2xl h-12"
                                >
                                    {isConfirmingDelete ? t('actions.cancel') : t('actions.edit')}
                                </Button>
                                <Button
                                    onClick={handleDeleteClick}
                                    disabled={deleteChannel.isPending}
                                    variant="destructive"
                                    className="w-full rounded-2xl h-12"
                                >
                                    <Trash2 className={`size-4 transition-transform ${isConfirmingDelete ? 'scale-110' : ''}`} />
                                    {deleteChannel.isPending
                                        ? t('actions.deleting')
                                        : isConfirmingDelete
                                            ? t('actions.confirmDelete')
                                            : t('actions.delete')}
                                </Button>
                            </div>
                        </TabsContent>

                        <TabsContent value="editing">
                            <ChannelForm
                                formData={formData}
                                onFormDataChange={setFormData}
                                onSubmit={handleUpdate}
                                isPending={updateChannel.isPending}
                                submitText={t('actions.save')}
                                pendingText={t('actions.saving')}
                                onCancel={() => setIsEditing(false)}
                                cancelText={t('actions.cancel')}
                                idPrefix="channel"
                            />
                        </TabsContent>
                    </TabsContents>
                </Tabs>
            </MorphingDialogDescription>
        </>
    );
}
