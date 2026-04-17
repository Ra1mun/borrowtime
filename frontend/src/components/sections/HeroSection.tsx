import { motion } from 'motion/react'

export default function HeroSection() {
    return (
        <section className="hero">
            <motion.div
                className="hero__logoBox"
                initial={{ opacity: 0, y: 40, scale: 0.96 }}
                animate={{ opacity: 1, y: 0, scale: 1 }}
                transition={{ duration: 0.9, ease: 'easeOut' }}
            >
                <h1 className="hero__title">BORROWED</h1>
            </motion.div>

            <motion.div
                className="hero__subtitle"
                initial={{ opacity: 0, y: 24 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.9, delay: 0.2, ease: 'easeOut' }}
            >
                <span>T</span>
                <span>I</span>
                <span>M</span>
                <span>E</span>
            </motion.div>
        </section>
    )
}