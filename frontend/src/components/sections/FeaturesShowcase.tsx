import { AnimatePresence, motion, useMotionValueEvent, useScroll } from 'motion/react'
import { useRef, useState, type ReactNode } from 'react'
import * as React from "react";

export type ShowcaseCircle = {
    className: string
}

export type ShowcaseSlide = {
    title: string
    text: string
    image: string
    circles?: ShowcaseCircle[]
    floatingContent?: ReactNode
    imageClassName?: string
    visualClassName?: string
    cardClassName?: string
    rightPaneClassName?: string
}

type FeaturesShowcaseProps = {
    slides: ShowcaseSlide[]
}

function clamp(value: number, min: number, max: number) {
    return Math.min(max, Math.max(min, value))
}

export default function FeaturesShowcase({ slides }: FeaturesShowcaseProps) {
    const rootRef = useRef<HTMLElement | null>(null)
    const [activeIndex, setActiveIndex] = useState(0)

    const { scrollYProgress } = useScroll({
        target: rootRef,
        offset: ['start start', 'end end'],
    })

    useMotionValueEvent(scrollYProgress, 'change', (latest) => {
        const nextIndex = clamp(Math.floor(latest * slides.length), 0, slides.length - 1)
        setActiveIndex((prev) => (prev === nextIndex ? prev : nextIndex))
    })

    const activeSlide = slides[activeIndex]

    return (
        <section
            ref={rootRef}
            className="showcase"
            style={{ ['--slides-count' as string]: slides.length } as React.CSSProperties}
        >
            <div className="showcase__sticky">
                <div className="showcase__shell">
                    <div className="showcase__pane showcase__pane--left" />
                    <div
                        className={`showcase__pane showcase__pane--right ${activeSlide.rightPaneClassName ?? ''}`}
                    />
                </div>

                <div className="showcase__content">
                    <div className="showcase__leftContent">
                        <AnimatePresence mode="wait">
                            <motion.div
                                key={`card-${activeIndex}`}
                                className="showcase__cardWrap"
                                initial={{ opacity: 0, x: -70, y: 18 }}
                                animate={{ opacity: 1, x: 0, y: 0 }}
                                exit={{ opacity: 0, x: -40, y: -10 }}
                                transition={{ duration: 0.55, ease: 'easeOut' }}
                            >
                                <div className={`showcase__card ${activeSlide.cardClassName ?? ''}`}>
                                    <h2>{activeSlide.title}</h2>
                                    <p>{activeSlide.text}</p>
                                </div>
                            </motion.div>
                        </AnimatePresence>
                    </div>

                    <div className="showcase__rightContent">
                        <AnimatePresence mode="wait">
                            <motion.div
                                key={`visual-${activeIndex}`}
                                className={`showcase__visual ${activeSlide.visualClassName ?? ''}`}
                                initial={{ opacity: 0, x: 70, y: 18 }}
                                animate={{ opacity: 1, x: 0, y: 0 }}
                                exit={{ opacity: 0, x: 40, y: -10 }}
                                transition={{ duration: 0.6, ease: 'easeOut' }}
                            >
                                {(activeSlide.circles ?? []).map((circle, index) => (
                                    <motion.span
                                        key={`${activeIndex}-${circle.className}-${index}`}
                                        className={`showcase__circle ${circle.className}`}
                                        initial={{ opacity: 0, scale: 0.85 }}
                                        animate={{ opacity: 1, scale: 1 }}
                                        exit={{ opacity: 0, scale: 0.85 }}
                                        transition={{ duration: 0.35, delay: index * 0.05 }}
                                    />
                                ))}

                                <motion.div
                                    className={`showcase__imageWrap ${activeSlide.imageClassName ?? ''}`}
                                    initial={{ opacity: 0, scale: 0.97 }}
                                    animate={{ opacity: 1, scale: 1 }}
                                    exit={{ opacity: 0, scale: 0.98 }}
                                    transition={{ duration: 0.55, ease: 'easeOut' }}
                                >
                                    <img src={activeSlide.image} alt={activeSlide.title} />
                                </motion.div>

                                {activeSlide.floatingContent && (
                                    <motion.div
                                        className="showcase__floatingLayer"
                                        initial={{ opacity: 0, y: 16 }}
                                        animate={{ opacity: 1, y: 0 }}
                                        exit={{ opacity: 0, y: 8 }}
                                        transition={{ duration: 0.45, delay: 0.12, ease: 'easeOut' }}
                                    >
                                        {activeSlide.floatingContent}
                                    </motion.div>
                                )}
                            </motion.div>
                        </AnimatePresence>
                    </div>
                </div>
            </div>
        </section>
    )
}