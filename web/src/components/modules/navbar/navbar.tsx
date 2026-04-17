"use client"

import { useState } from "react"
import { motion } from "motion/react"
import { cn } from "@/lib/utils"
import { useNavStore, type NavItem } from "@/components/modules/navbar"
import { ROUTES } from "@/route/config"
import { usePreload } from "@/route/use-preload"
import { ENTRANCE_VARIANTS } from "@/lib/animations/fluid-transitions"
import { useSettingStore } from "@/stores/setting"
import { BookOpen } from "lucide-react"
import { DocModal } from "./DocModal"
import { useTranslations } from "next-intl"

export function NavBar() {
    const { activeItem, setActiveItem } = useNavStore()
    const { preload } = usePreload()
    const hiddenNavItems = useSettingStore((s) => s.hiddenNavItems)
    const [isDocOpen, setIsDocOpen] = useState(false)
    const t = useTranslations('navbar')

    const visibleRoutes = ROUTES.filter((route) => !hiddenNavItems.includes(route.id))

    return (
        <div className="relative z-50 md:min-h-screen">
            <motion.nav
                aria-label="Main Navigation"
                className={cn(
                    "fixed bottom-6 left-1/2 -translate-x-1/2 flex items-center gap-1 p-3",
                    "md:sticky md:top-30 md:left-auto md:bottom-auto md:translate-x-0 md:flex-col md:gap-3",
                    "bg-sidebar text-sidebar-foreground border border-sidebar-border rounded-3xl",
                    "custom-shadow"
                )}
                variants={ENTRANCE_VARIANTS.navbar}
                initial="initial"
                animate="animate"
            >
                {visibleRoutes.map((route, index) => {
                    const isActive = activeItem === route.id
                    return (
                        <motion.button
                            key={route.id}
                            type="button"
                            onClick={() => setActiveItem(route.id as NavItem)}
                            onMouseEnter={() => preload(route.id)}
                            className={cn(
                                "relative p-2 md:p-3 rounded-2xl z-20",
                                isActive ? "text-sidebar-primary-foreground" : "text-sidebar-foreground/60 hover:bg-sidebar-accent"
                            )}
                            initial={{ opacity: 0, scale: 0.8 }}
                            animate={{
                                opacity: 1,
                                scale: 1,
                                transition: {
                                    delay: index * 0.05,
                                    duration: 0.3,
                                }
                            }}
                            whileHover={{ scale: 1.1, zIndex: 30 }}
                            whileTap={{ scale: 0.95 }}
                        >
                            {isActive && (
                                <motion.div
                                    layoutId="navbar-indicator"
                                    className="absolute inset-0 bg-sidebar-primary rounded-2xl z-0"
                                    transition={{ type: "spring", stiffness: 300, damping: 30 }}
                                />
                            )}
                            <span className="relative z-10">
                                <route.icon strokeWidth={2} />
                            </span>
                        </motion.button>
                    )
                })}

                <div className="w-px h-6 md:h-px md:w-6 bg-sidebar-border/50 mx-auto" />

                <motion.button
                    type="button"
                    onClick={() => setIsDocOpen(true)}
                    className={cn(
                        "relative p-2 md:p-3 rounded-2xl z-20",
                        "text-sidebar-foreground/60 hover:bg-sidebar-accent"
                    )}
                    title={t('doc')}
                    initial={{ opacity: 0, scale: 0.8 }}
                    animate={{
                        opacity: 1,
                        scale: 1,
                        transition: { delay: visibleRoutes.length * 0.05 + 0.1, duration: 0.3 }
                    }}
                    whileHover={{ scale: 1.1, zIndex: 30 }}
                    whileTap={{ scale: 0.95 }}
                >
                    <BookOpen strokeWidth={2} className="h-5 w-5" />
                </motion.button>
            </motion.nav>

            <DocModal
                isOpen={isDocOpen}
                onClose={() => setIsDocOpen(false)}
                onGoSetting={() => setActiveItem('setting')}
            />
        </div>
    )
}
