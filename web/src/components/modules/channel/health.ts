import { cn } from '@/lib/utils';
import type { ChannelHealthRoute, ChannelHealthSummary, ChannelKeyHealthSummary } from '@/api/endpoints/channel';

type HealthSummaryLike = Pick<ChannelHealthSummary, 'status' | 'best_ordering_score' | 'worst_raw_score' | 'cooldown_remaining_ms' | 'last_failure_kind' | 'tracked_routes' | 'cooling_routes' | 'warmup_routes'>
    | Pick<ChannelKeyHealthSummary, 'status' | 'best_ordering_score' | 'worst_raw_score' | 'cooldown_remaining_ms' | 'last_failure_kind' | 'tracked_routes' | 'cooling_routes' | 'warmup_routes'>;

export function getHealthTone(status?: string | null) {
    switch (status) {
        case 'healthy':
            return 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400';
        case 'cooling':
            return 'bg-red-500/15 text-red-700 dark:text-red-400';
        case 'degraded':
            return 'bg-amber-500/15 text-amber-700 dark:text-amber-400';
        case 'warming':
            return 'bg-sky-500/15 text-sky-700 dark:text-sky-400';
        case 'neutral':
            return 'bg-muted text-muted-foreground';
        default:
            return 'bg-secondary text-secondary-foreground';
    }
}

export function formatCooldown(ms?: number | null) {
    if (!ms || ms <= 0) return '0s';
    const totalSeconds = Math.ceil(ms / 1000);
    if (totalSeconds < 60) return `${totalSeconds}s`;
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;
    if (minutes < 60) return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`;
    const hours = Math.floor(minutes / 60);
    const restMinutes = minutes % 60;
    return restMinutes > 0 ? `${hours}h ${restMinutes}m` : `${hours}h`;
}

export function formatSignedScore(score: number) {
    return score > 0 ? `+${score}` : `${score}`;
}

export function getDisplaySummaryScore(summary?: HealthSummaryLike | null) {
    if (!summary) return 0;
    if (summary.status === 'healthy') return summary.best_ordering_score;
    if (summary.status === 'cooling' || summary.status === 'degraded') return summary.worst_raw_score;
    return summary.best_ordering_score;
}

export function getRouteDisplayScore(route: ChannelHealthRoute) {
    if (route.warmup_pending) return route.raw_score;
    if (route.state === 'open') return route.raw_score;
    return route.ordering_score;
}

export function humanizeFailureKind(kind?: string | null) {
    if (!kind || kind === 'unknown') return 'unknown';
    return kind.replaceAll('_', ' ');
}

export function healthBadgeClassName(status?: string | null, extra?: string) {
    return cn('h-5 px-1.5 text-[10px]', getHealthTone(status), extra);
}
