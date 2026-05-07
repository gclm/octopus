import { useState } from 'react';
import {
    MorphingDialogClose,
    MorphingDialogTitle,
    MorphingDialogDescription,
    useMorphingDialog,
} from '@/components/ui/morphing-dialog';
import { useCreateChannel, OutboundType, AutoGroupType, buildTestPreviewRequest } from '@/api/endpoints/channel';
import { useTranslations } from 'next-intl';
import { ChannelForm, type ChannelFormData } from './Form';
import { TestDialog } from './TestDialog';
import { API_BASE_URL } from '@/api/client';

export function CreateDialogContent() {
    const { setIsOpen } = useMorphingDialog();
    const createChannel = useCreateChannel();
    const [testBody, setTestBody] = useState<object | null>(null);
    const [formData, setFormData] = useState<ChannelFormData>({
        name: '',
        endpoints: [{ type: OutboundType.OpenAIChat, base_url: '', enabled: true }],
        custom_header: [],
        channel_proxy: '',
        param_override: '',
        keys: [{ enabled: true, channel_key: '', remark: '' }],
        model: '',
        custom_model: '',
        auto_sync: false,
        auto_group: AutoGroupType.None,
        enabled: true,
        proxy: false,
        match_regex: '',
    });
    const t = useTranslations('channel.create');
    const tActions = useTranslations('channel.detail.actions');

    const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        const normalizedEndpoints = (formData.endpoints ?? [])
            .filter((ep) => ep.base_url.trim())
            .map((ep) => ({
                type: ep.type,
                base_url: ep.base_url.trim(),
                enabled: ep.enabled,
            }));
        const normalizedKeys = formData.keys
            .filter((k) => k.channel_key.trim())
            .map((k) => ({ enabled: k.enabled, channel_key: k.channel_key, remark: k.remark ?? '' }));
        const normalizedHeaders = (formData.custom_header ?? [])
            .map((h) => ({ header_key: h.header_key.trim(), header_value: h.header_value }))
            .filter((h) => h.header_key && h.header_value !== '');

        const channelProxy = formData.channel_proxy.trim();
        const paramOverride = formData.param_override.trim();
        createChannel.mutate(
            {
                name: formData.name,
                endpoints: normalizedEndpoints,
                enabled: formData.enabled,
                keys: normalizedKeys,
                model: formData.model,
                custom_model: formData.custom_model,
                proxy: formData.proxy,
                auto_sync: formData.auto_sync,
                auto_group: formData.auto_group,
                custom_header: normalizedHeaders,
                channel_proxy: channelProxy,
                param_override: paramOverride,
                match_regex: formData.match_regex.trim(),
            },
            {
                onSuccess: () => {
                    setFormData({
                        name: '',
                        endpoints: [{ type: OutboundType.OpenAIChat, base_url: '', enabled: true }],
                        custom_header: [],
                        channel_proxy: '',
                        param_override: '',
                        keys: [{ enabled: true, channel_key: '', remark: '' }],
                        model: '',
                        custom_model: '',
                        auto_sync: false,
                        auto_group: AutoGroupType.None,
                        enabled: true,
                        proxy: false,
                        match_regex: '',
                    });
                    setIsOpen(false);
                }
            });
    };

    const handleTest = () => {
        const body = buildTestPreviewRequest(formData);
        if (!body) return;
        setTestBody(body);
    };

    return (
        <div className="w-screen max-w-full md:max-w-xl h-full min-h-0 flex flex-col">
            <MorphingDialogTitle className="shrink-0">
                <header className="mb-6 flex items-center justify-between">
                    <h2 className="text-2xl font-bold text-card-foreground">{t('dialogTitle')}</h2>
                    <MorphingDialogClose
                        className="relative right-0 top-0"
                        variants={{
                            initial: { opacity: 0, scale: 0.8 },
                            animate: { opacity: 1, scale: 1 },
                            exit: { opacity: 0, scale: 0.8 }
                        }}
                    />
                </header>
            </MorphingDialogTitle>
            <MorphingDialogDescription disableLayoutAnimation className="flex-1 min-h-0 overflow-auto">
                <ChannelForm
                    formData={formData}
                    onFormDataChange={setFormData}
                    onSubmit={handleSubmit}
                    isPending={createChannel.isPending}
                    submitText={t('submit')}
                    pendingText={t('submitting')}
                    idPrefix="new-channel"
                    onTest={handleTest}
                    testText={tActions('test')}
                />
            </MorphingDialogDescription>

            <TestDialog
                open={testBody !== null}
                onOpenChange={(open) => { if (!open) setTestBody(null); }}
                streamUrl={`${API_BASE_URL}/api/v1/channel/test-stream`}
                body={testBody}
            />
        </div>
    );
}
