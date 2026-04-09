import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type Locale = 'zh_hans' | 'zh_hant' | 'en';

interface SettingState {
    locale: Locale;
    setLocale: (locale: Locale) => void;
    hiddenNavItems: string[];
    toggleNavItem: (id: string) => void;
    isNavItemHidden: (id: string) => boolean;
}

export const useSettingStore = create<SettingState>()(
    persist(
        (set, get) => ({
            locale: 'zh_hans',
            setLocale: (locale) => set({ locale }),
            hiddenNavItems: [],
            toggleNavItem: (id) => {
                if (id === 'setting') return;
                const { hiddenNavItems } = get();
                set({
                    hiddenNavItems: hiddenNavItems.includes(id)
                        ? hiddenNavItems.filter((item) => item !== id)
                        : [...hiddenNavItems, id],
                });
            },
            isNavItemHidden: (id) => get().hiddenNavItems.includes(id),
        }),
        {
            name: 'octopus-settings',
        }
    )
);
